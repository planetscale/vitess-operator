/*
Copyright 2024 PlanetScale Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vitessbackupschedule

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"math/rand/v2"
	"strconv"
	"strings"

	"sort"
	"time"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	"planetscale.dev/vitess-operator/pkg/controller/vitessshard"
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/resync"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerName = "vitessbackupschedule-controller"
)

var (
	maxConcurrentReconciles = flag.Int("vitessbackupschedule_concurrent_reconciles", 10, "the maximum number of different vitessbackupschedule resources to reconcile concurrently")
	resyncPeriod            = flag.Duration("vitessbackupschedule_resync_period", 1*time.Minute, "reconcile vitessbackupschedules with this period even if no Kubernetes events occur")

	scheduledTimeAnnotation = "planetscale.com/backup-scheduled-at"

	log = logrus.WithField("controller", "VitessBackupSchedule")
)

// watchResources should contain all the resource types that this controller creates.
var watchResources = []client.Object{
	&kbatch.Job{},
}

type (
	// ReconcileVitessBackupsSchedule reconciles a VitessBackupSchedule object
	ReconcileVitessBackupsSchedule struct {
		client     client.Client
		scheme     *runtime.Scheme
		resync     *resync.Periodic
		recorder   record.EventRecorder
		reconciler *reconciler.Reconciler
	}

	jobsList struct {
		active     []*kbatch.Job
		successful []*kbatch.Job
		failed     []*kbatch.Job
	}
)

var _ reconcile.Reconciler = &ReconcileVitessBackupsSchedule{}

// Add creates a new Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (*ReconcileVitessBackupsSchedule, error) {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetEventRecorderFor(controllerName)

	return &ReconcileVitessBackupsSchedule{
		client:     c,
		scheme:     scheme,
		resync:     resync.NewPeriodic(controllerName, *resyncPeriod),
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileVitessBackupsSchedule) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: *maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessBackupSchedule
	if err := c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessBackupSchedule{}, &handler.TypedEnqueueRequestForObject[*planetscalev2.VitessBackupSchedule]{})); err != nil {
		return err
	}

	// Watch for changes to kbatch.Job and requeue the owner VitessBackupSchedule.
	for _, resource := range watchResources {
		err := c.Watch(source.Kind(mgr.GetCache(), resource, handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&planetscalev2.VitessBackupStorage{},
			handler.OnlyControllerOwner(),
		)))
		if err != nil {
			return err
		}
	}

	// Periodically resync even when no Kubernetes events have come in.
	if err := c.Watch(r.resync.WatchSource()); err != nil {
		return err
	}

	return nil
}

// Reconcile implements the kubernetes Reconciler interface.
// The main goal of this function is to create new Job k8s object according to the VitessBackupSchedule schedule.
// It also takes care of removing old failed and successful jobs, given the settings of VitessBackupSchedule.
// The function is structured as follows:
//   - Get the VitessBackupSchedule object
//   - List all jobs and define the last scheduled Job
//   - Clean up old Job objects
//   - Create a new Job if needed
//   - Update the VitessBackupSchedule Status
func (r *ReconcileVitessBackupsSchedule) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	log = log.WithFields(logrus.Fields{
		"namespace":            req.Namespace,
		"VitessBackupSchedule": req.Name,
	})
	log.Info("Reconciling VitessBackupSchedule")

	var vbsc planetscalev2.VitessBackupSchedule
	if err = r.client.Get(ctx, req.NamespacedName, &vbsc); err != nil {
		log.WithError(err).Error(" unable to fetch VitessBackupSchedule")
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// If the Suspend setting is set to true, we can skip adding any job, our work is done here.
	if vbsc.Spec.Suspend != nil && *vbsc.Spec.Suspend {
		log.Info("VitessBackupSchedule suspended, skipping")
		return ctrl.Result{}, nil
	}

	oldStatus := vbsc.DeepCopy()
	vbsc.Status = planetscalev2.NewVitessBackupScheduleStatus(vbsc.Status)

	// Register this reconciling attempt no matter if we fail or succeed.
	defer func() {
		reconcileCount.WithLabelValues(vbsc.Name, metrics.Result(err)).Inc()
	}()

	resultBuilder := &results.Builder{}
	_, _ = resultBuilder.Merge(r.reconcileStrategies(ctx, req, vbsc))

	if !apiequality.Semantic.DeepEqual(&vbsc.Status, &oldStatus) {
		if err := r.client.Status().Update(ctx, &vbsc); err != nil {
			if !apierrors.IsConflict(err) {
				log.WithError(err).Error("unable to update VitessBackupSchedule status")
			}
			_, _ = resultBuilder.Error(err)
		}
	}

	// Request a periodic resync of this VitessBackupSchedule object to check
	// if existing Jobs have finished or timed out and need to be cleaned up
	// even if no Kubernetes events have occurred.
	r.resync.Enqueue(req.NamespacedName)

	return resultBuilder.Result()
}

func (r *ReconcileVitessBackupsSchedule) reconcileStrategies(ctx context.Context, req ctrl.Request, vbsc planetscalev2.VitessBackupSchedule) (ctrl.Result, error) {
	resultBuilder := &results.Builder{}

	for _, strategy := range vbsc.Spec.Strategy {
		_, _ = resultBuilder.Merge(r.reconcileStrategy(ctx, strategy, req, vbsc))
	}
	return resultBuilder.Result()
}

func (r *ReconcileVitessBackupsSchedule) reconcileStrategy(
	ctx context.Context,
	strategy planetscalev2.VitessBackupScheduleStrategy,
	req ctrl.Request,
	vbsc planetscalev2.VitessBackupSchedule,
) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	start, end, ok := strings.Cut(strategy.Shard, "-")
	if !ok {
		return resultBuilder.Error(fmt.Errorf("invalid strategy shard: %s", strategy.Shard))
	}
	vkr := planetscalev2.VitessKeyRange{
		Start: start,
		End:   end,
	}
	jobs, mostRecentTime, err := r.getJobsList(ctx, req, vbsc, strategy.Keyspace, vkr.SafeName())
	if err != nil {
		// We had an error reading the jobs, we can requeue.
		return resultBuilder.Error(err)
	}

	// We must clean up old jobs to not overcrowd the number of Pods and Jobs in the cluster.
	// This will be done according to both failedJobsHistoryLimit and successfulJobsHistoryLimit fields.
	r.cleanupJobsWithLimit(ctx, jobs.failed, vbsc.GetFailedJobsLimit())
	r.cleanupJobsWithLimit(ctx, jobs.successful, vbsc.GetSuccessfulJobsLimit())

	err = r.removeTimeoutJobs(ctx, jobs.active, vbsc.Name, vbsc.Spec.JobTimeoutMinutes)
	if err != nil {
		// We had an error while removing timed out jobs, we can requeue
		return resultBuilder.Error(err)
	}

	missedRun, nextRun, err := getNextSchedule(vbsc, time.Now(), mostRecentTime)
	if err != nil {
		log.Error(err, "unable to figure out VitessBackupSchedule schedule")
		// Re-queuing here does not make sense as we have an error with the schedule and the user needs to fix it first.
		return resultBuilder.Error(reconcile.TerminalError(err))
	}

	// If we did not miss any run, we can skip and not requeue anything
	if missedRun.IsZero() {
		return resultBuilder.Result()
	}

	// Keep track of when we need to requeue this job
	requeueAfter := nextRun.Sub(time.Now())
	_, _ = resultBuilder.RequeueAfter(requeueAfter)

	// Check whether we are too late to create this Job or not. The startingDeadlineSeconds field will help us
	// schedule Jobs that are late.
	tooLate := false
	if vbsc.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*vbsc.Spec.StartingDeadlineSeconds) * time.Second).Before(time.Now())
	}
	if tooLate {
		log.Infof("missed starting deadline for latest run; skipping; next run is scheduled for: %s", nextRun.Format(time.RFC3339))
		return resultBuilder.Result()
	}

	// Check concurrency policy and skip this job if we have ForbidConcurrent set plus an active job
	if vbsc.Spec.ConcurrencyPolicy == planetscalev2.ForbidConcurrent && len(jobs.active) > 0 {
		log.Infof("concurrency policy blocks concurrent runs: skipping, number of active jobs: %d", len(jobs.active))
		return resultBuilder.Result()
	}

	// Now that the different policies are checked, we can create and apply our new job.
	job, err := r.createJob(ctx, vbsc, strategy, missedRun, vkr)
	if err != nil {
		// Re-queuing here does not make sense as we have an error with the template and the user needs to fix it first.
		log.WithError(err).Error("unable to construct job from template")
		return resultBuilder.Error(reconcile.TerminalError(err))
	}

	if err = r.client.Create(ctx, job); err != nil {
		// if the job already exists it means another reconciling loop created the job since we last fetched
		// the list of jobs to create, we can safely return without failing.
		if apierrors.IsAlreadyExists(err) {
			log.Infof("job %s already exists, will retry in %s", job.Name, requeueAfter.String())
			return resultBuilder.Result()
		}
		// Simply re-queue here
		return resultBuilder.Error(err)
	}
	log.Infof("created new job: %s, next job scheduled in %s", job.Name, requeueAfter.String())
	vbsc.Status.LastScheduledTimes[strategy.Name] = &metav1.Time{Time: missedRun}
	return resultBuilder.Result()
}

func getNextSchedule(vbsc planetscalev2.VitessBackupSchedule, now time.Time, mostRecentTime *time.Time) (time.Time, time.Time, error) {
	sched, err := cron.ParseStandard(vbsc.Spec.Schedule)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("unable to parse schedule %q: %v", vbsc.Spec.Schedule, err)
	}

	// Set the last scheduled time by either looking at the VitessBackupSchedule's Status or
	// by looking at its creation time.
	if mostRecentTime == nil {
		mostRecentTime = &vbsc.ObjectMeta.CreationTimestamp.Time
	}

	if vbsc.Spec.StartingDeadlineSeconds != nil {
		// controller is not going to schedule anything below this point
		schedulingDeadline := now.Add(-time.Second * time.Duration(*vbsc.Spec.StartingDeadlineSeconds))

		if schedulingDeadline.After(*mostRecentTime) {
			*mostRecentTime = schedulingDeadline
		}
	}

	// Next schedule is later, simply return the next scheduled time.
	if mostRecentTime.After(now) {
		return time.Time{}, sched.Next(now), nil
	}

	var lastMissed time.Time
	missedRuns := 0
	for t := sched.Next(*mostRecentTime); !t.After(now); t = sched.Next(t) {
		lastMissed = t
		missedRuns++

		// If we have too many missed jobs, just bail out as the clock lag is too big
		if missedRuns > vbsc.GetMissedRunsLimit() {
			return time.Time{}, time.Time{}, fmt.Errorf("too many missed runs, check clock skew or increase .spec.allowedMissedRun")
		}
	}

	return lastMissed, sched.Next(now), nil
}

// getJobsList fetches all existing Jobs in the cluster and return them by categories: active, failed or successful.
// It also returns at what time was the last job created, which is needed to update VitessBackupSchedule's status,
// and plan future jobs.
func (r *ReconcileVitessBackupsSchedule) getJobsList(
	ctx context.Context,
	req ctrl.Request,
	vbsc planetscalev2.VitessBackupSchedule,
	keyspace string,
	shardSafeName string,
) (jobsList, *time.Time, error) {
	var existingJobs kbatch.JobList

	err := r.client.List(ctx, &existingJobs, client.InNamespace(req.Namespace), client.MatchingLabels{
		planetscalev2.BackupScheduleLabel: vbsc.Name,
		planetscalev2.ClusterLabel:        vbsc.Spec.Cluster,
		planetscalev2.KeyspaceLabel:       keyspace,
		planetscalev2.ShardLabel:          shardSafeName,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		log.WithError(err).Error("unable to list Jobs in cluster")
		return jobsList{}, nil, err
	}

	var jobs jobsList

	var mostRecentTime *time.Time

	for i, job := range existingJobs.Items {
		_, jobType := isJobFinished(&job)
		switch jobType {
		case kbatch.JobFailed, kbatch.JobFailureTarget:
			jobs.failed = append(jobs.failed, &existingJobs.Items[i])
		case kbatch.JobComplete:
			jobs.successful = append(jobs.successful, &existingJobs.Items[i])
		case kbatch.JobSuspended, "":
			jobs.active = append(jobs.active, &existingJobs.Items[i])
		default:
			return jobsList{}, nil, fmt.Errorf("unknown job type: %s", jobType)
		}

		scheduledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			log.WithError(err).Errorf("unable to parse schedule time for existing job, found: %s", job.Annotations[scheduledTimeAnnotation])
			continue
		}
		if scheduledTimeForJob != nil && (mostRecentTime == nil || mostRecentTime.Before(*scheduledTimeForJob)) {
			mostRecentTime = scheduledTimeForJob
		}
	}
	return jobs, mostRecentTime, nil
}

// cleanupJobsWithLimit removes all Job objects from the cluster ordered by oldest to newest and
// respecting the given limit, keeping minimum "limit" jobs in the cluster.
func (r *ReconcileVitessBackupsSchedule) cleanupJobsWithLimit(ctx context.Context, jobs []*kbatch.Job, limit int32) {
	if limit == -1 {
		return
	}

	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].Status.StartTime == nil {
			return jobs[j].Status.StartTime != nil
		}
		return jobs[i].Status.StartTime.Before(jobs[j].Status.StartTime)
	})

	for i, job := range jobs {
		if int32(i) >= int32(len(jobs))-limit {
			break
		}
		// delete the job
		if err := r.client.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
			log.WithError(err).Errorf("unable to delete old job: %s", job.Name)
		} else {
			log.Infof("deleted old job: %s", job.Name)
		}

		// delete the vtbackup pod's PVC
		pvc := &corev1.PersistentVolumeClaim{}
		err := r.client.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: job.Name}, pvc)
		if err != nil {
			log.WithError(err).Errorf("unable to get PVC for job: %s", job.Name)
			if apierrors.IsNotFound(err) {
				continue
			}
		}
		if err := r.client.Delete(ctx, pvc, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
			log.WithError(err).Errorf("unable to delete old PVC for job: %s", job.Name)
		} else {
			log.Infof("deleted old PVC for job: %s", job.Name)
		}
	}
}

func (r *ReconcileVitessBackupsSchedule) removeTimeoutJobs(ctx context.Context, jobs []*kbatch.Job, vbscName string, timeout int32) error {
	if timeout == -1 {
		return nil
	}
	for _, job := range jobs {
		jobStartTime, err := getScheduledTimeForJob(job)
		if err != nil {
			return err
		}
		if jobStartTime.Add(time.Minute * time.Duration(timeout)).Before(time.Now()) {
			if err = r.client.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
				log.WithError(err).Errorf("unable to delete timed out job: %s", job.Name)
			} else {
				log.Infof("deleted timed out job: %s", job.Name)
			}
			timeoutJobsCount.WithLabelValues(vbscName, metrics.Result(err)).Inc()

			pvc := &corev1.PersistentVolumeClaim{}
			err := r.client.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: job.Name}, pvc)
			if err != nil {
				log.WithError(err).Errorf("unable to get PVC for timed out job: %s", job.Name)
				if apierrors.IsNotFound(err) {
					continue
				}
			}
			if err := r.client.Delete(ctx, pvc, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
				log.WithError(err).Errorf("unable to delete old PVC for timed out job: %s", job.Name)
			} else {
				log.Infof("deleted old PVC for timed out job: %s", job.Name)
			}
		}
	}
	return nil
}

func isJobFinished(job *kbatch.Job) (bool, kbatch.JobConditionType) {
	for _, c := range job.Status.Conditions {
		if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}

	return false, ""
}

func getScheduledTimeForJob(job *kbatch.Job) (*time.Time, error) {
	timeRaw := job.Annotations[scheduledTimeAnnotation]
	if len(timeRaw) == 0 {
		return nil, nil
	}

	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return nil, err
	}

	return &timeParsed, nil
}

func (r *ReconcileVitessBackupsSchedule) createJob(
	ctx context.Context,
	vbsc planetscalev2.VitessBackupSchedule,
	strategy planetscalev2.VitessBackupScheduleStrategy,
	scheduledTime time.Time,
	vkr planetscalev2.VitessKeyRange,
) (*kbatch.Job, error) {
	name := names.JoinWithConstraints(names.ServiceConstraints, vbsc.Name, strategy.Keyspace, vkr.SafeName(), strconv.Itoa(int(scheduledTime.Unix())))

	labels := map[string]string{
		planetscalev2.BackupScheduleLabel: vbsc.Name,
		planetscalev2.ClusterLabel:        vbsc.Spec.Cluster,
		planetscalev2.KeyspaceLabel:       strategy.Keyspace,
		planetscalev2.ShardLabel:          vkr.SafeName(),
	}

	meta := metav1.ObjectMeta{
		Labels:      labels,
		Annotations: make(map[string]string),
		Name:        name,
		Namespace:   vbsc.Namespace,
	}
	maps.Copy(meta.Annotations, vbsc.Annotations)
	maps.Copy(meta.Annotations, vbsc.Spec.Annotations)

	meta.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)

	maps.Copy(meta.Labels, vbsc.Labels)

	pod, vtbackupSpec, err := r.createJobPod(ctx, vbsc, strategy, name, vkr, labels)
	if err != nil {
		return nil, err
	}
	job := &kbatch.Job{
		ObjectMeta: meta,
		Spec: kbatch.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec:       pod.Spec,
			},
		},
	}

	if err := ctrl.SetControllerReference(&vbsc, job, r.scheme); err != nil {
		return nil, err
	}

	// Create the corresponding PVC for the new vtbackup pod
	pvc := &corev1.PersistentVolumeClaim{}
	key := client.ObjectKey{
		Namespace: job.Namespace,
		Name:      name,
	}
	err = r.client.Get(ctx, key, pvc)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		newPVC := vttablet.NewPVC(key, vtbackupSpec.TabletSpec)
		if err := ctrl.SetControllerReference(&vbsc, newPVC, r.scheme); err != nil {
			return nil, err
		}
		err = r.client.Create(ctx, newPVC)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("PVC already exists for job %s", job.Name)
	}
	return job, nil
}

func (r *ReconcileVitessBackupsSchedule) createJobPod(
	ctx context.Context,
	vbsc planetscalev2.VitessBackupSchedule,
	strategy planetscalev2.VitessBackupScheduleStrategy,
	name string,
	vkr planetscalev2.VitessKeyRange,
	labels map[string]string,
) (pod *corev1.Pod, spec *vttablet.BackupSpec, err error) {
	vts, err := r.getShardFromKeyspace(ctx, vbsc.Namespace, vbsc.Spec.Cluster, strategy.Keyspace, strategy.Shard)
	if err != nil {
		return nil, nil, err
	}

	_, completedBackups, err := vitessbackup.GetBackups(ctx, vbsc.Namespace, vbsc.Spec.Cluster, strategy.Keyspace, vkr.SafeName(),
		func(ctx context.Context, allBackupsList *planetscalev2.VitessBackupList, listOpts *client.ListOptions) error {
			return r.client.List(ctx, allBackupsList, listOpts)
		},
	)
	if err != nil {
		return nil, nil, err
	}

	backupType := vitessbackup.TypeUpdate
	if len(completedBackups) == 0 {
		if vts.Status.HasMaster == corev1.ConditionTrue {
			return nil, nil, fmt.Errorf("this shard has 0 backup and a running primary, the schedule cannot create an empty backup, please create a backup manually first")
		} else {
			backupType = vitessbackup.TypeInit
		}
	}

	podKey := client.ObjectKey{
		Namespace: vbsc.Namespace,
		Name:      name,
	}
	vtbackupSpec := vitessshard.MakeVtbackupSpec(podKey, &vts, labels, backupType)
	p := vttablet.NewBackupPod(podKey, vtbackupSpec, vts.Spec.Images.Mysqld.Image())

	// Explicitly do not restart on failure. The VitessBackupSchedule controller will retry the failed job
	// during the next scheduled run.
	p.Spec.RestartPolicy = corev1.RestartPolicyNever

	p.Spec.Affinity = vbsc.Spec.Affinity
	return p, vtbackupSpec, nil
}

func (r *ReconcileVitessBackupsSchedule) getVtctldServiceName(ctx context.Context, vbsc *planetscalev2.VitessBackupSchedule, cluster string) (svcName string, svcPort int32, err error) {
	svcList := &corev1.ServiceList{}
	listOpts := &client.ListOptions{
		Namespace: vbsc.Namespace,
		LabelSelector: apilabels.Set{
			planetscalev2.ClusterLabel:   cluster,
			planetscalev2.ComponentLabel: planetscalev2.VtctldComponentName,
		}.AsSelector(),
	}
	if err = r.client.List(ctx, svcList, listOpts); err != nil {
		return "", 0, fmt.Errorf("unable to list vtctld service in %q: %v", vbsc.Namespace, err)
	}

	if len(svcList.Items) > 0 {
		service := svcList.Items[rand.IntN(len(svcList.Items))]
		svcName = service.Name
		for _, port := range service.Spec.Ports {
			if port.Name == planetscalev2.DefaultGrpcPortName {
				svcPort = port.Port
				break
			}
		}
	}

	if svcName == "" || svcPort == 0 {
		return "", 0, fmt.Errorf("no vtctld service found in %q namespace", vbsc.Namespace)
	}
	return svcName, svcPort, nil
}

func (r *ReconcileVitessBackupsSchedule) getShardFromKeyspace(ctx context.Context, namespace, cluster, keyspace, shard string) (planetscalev2.VitessShard, error) {
	shardsList := &planetscalev2.VitessShardList{}
	listOpts := &client.ListOptions{
		Namespace: namespace,
		LabelSelector: apilabels.Set{
			planetscalev2.ClusterLabel:  cluster,
			planetscalev2.KeyspaceLabel: keyspace,
		}.AsSelector(),
	}
	if err := r.client.List(ctx, shardsList, listOpts); err != nil {
		return planetscalev2.VitessShard{}, fmt.Errorf("unable to list shards of keyspace %s in %s: %v", keyspace, namespace, err)
	}
	for _, item := range shardsList.Items {
		if item.Spec.KeyRange.String() == shard {
			return item, nil
		}
	}
	return planetscalev2.VitessShard{}, fmt.Errorf("unable to find shard %s in keyspace %s in %s", shard, keyspace, namespace)
}

type keyspace struct {
	name   string
	shards []string
}

func (r *ReconcileVitessBackupsSchedule) getAllShardsInCluster(ctx context.Context, namespace, cluster string) ([]keyspace, error) {
	ksList := &planetscalev2.VitessKeyspaceList{}
	listOpts := &client.ListOptions{
		Namespace: namespace,
		LabelSelector: apilabels.Set{
			planetscalev2.ClusterLabel: cluster,
		}.AsSelector(),
	}
	if err := r.client.List(ctx, ksList, listOpts); err != nil {
		return nil, fmt.Errorf("unable to list shards in namespace %s: %v", namespace, err)
	}
	result := make([]keyspace, 0, len(ksList.Items))
	for _, item := range ksList.Items {
		ks := keyspace{
			name: item.Spec.Name,
		}
		for shardName := range item.Status.Shards {
			ks.shards = append(ks.shards, shardName)
		}
		if len(ks.shards) > 0 {
			result = append(result, ks)
		}
	}
	return result, nil
}
