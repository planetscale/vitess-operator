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

	"sort"
	"time"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ref "k8s.io/client-go/tools/reference"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerName   = "vitessbackupschedule-controller"
	vtctldclientPath = "/vt/bin/vtctldclient"
)

var (
	maxConcurrentReconciles = flag.Int("vitessbackupschedule_concurrent_reconciles", 10, "the maximum number of different vitessbackupschedule to reconcile concurrently")

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
	if err := c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessBackupSchedule{}), &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to kbatch.Job and requeue the owner VitessBackupSchedule.
	for _, resource := range watchResources {
		err := c.Watch(source.Kind(mgr.GetCache(), resource), handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&planetscalev2.VitessBackupStorage{},
			handler.OnlyControllerOwner(),
		))
		if err != nil {
			return err
		}
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
func (r *ReconcileVitessBackupsSchedule) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	resultBuilder := &results.Builder{}

	log = log.WithFields(logrus.Fields{
		"namespace":            req.Namespace,
		"VitessBackupSchedule": req.Name,
	})
	log.Info("Reconciling VitessBackupSchedule")

	var vbsc planetscalev2.VitessBackupSchedule
	if err := r.client.Get(ctx, req.NamespacedName, &vbsc); err != nil {
		log.WithError(err).Error(" unable to fetch VitessBackupSchedule")
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return resultBuilder.Result()
		}
		// Error reading the object - requeue the request.
		return resultBuilder.Error(err)
	}

	jobs, mostRecentTime, err := r.getJobsList(ctx, req, vbsc.Name)
	if err != nil {
		// We had an error reading the jobs, we can requeue.
		return resultBuilder.Error(err)
	}

	err = r.updateVitessBackupScheduleStatus(ctx, mostRecentTime, vbsc, jobs.active)
	if err != nil {
		// We had an error updating the status, we can requeue.
		return resultBuilder.Error(err)
	}

	// We must clean up old jobs to not overcrowd the number of Pods and Jobs in the cluster.
	// This will be done according to both failedJobsHistoryLimit and successfulJobsHistoryLimit fields.
	r.cleanupJobsWithLimit(ctx, jobs.failed, vbsc.GetFailedJobsLimit())
	r.cleanupJobsWithLimit(ctx, jobs.successful, vbsc.GetSuccessfulJobsLimit())

	// If the Suspend setting is set to true, we can skip adding any job, our work is done here.
	if vbsc.Spec.Suspend != nil && *vbsc.Spec.Suspend {
		log.Info("VitessBackupSchedule suspended, skipping")
		return ctrl.Result{}, nil
	}

	missedRun, nextRun, err := getNextSchedule(vbsc, time.Now())
	if err != nil {
		log.Error(err, "unable to figure out VitessBackupSchedule schedule")
		// Re-queuing here does not make sense as we have an error with the schedule and the user needs to fix it first.
		return ctrl.Result{}, nil
	}

	// Ask kubernetes to re-queue for the next scheduled job, and skip if we don't miss any run.
	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(time.Now())}
	if missedRun.IsZero() {
		return scheduledResult, nil
	}

	// Check whether we are too late to create this Job or not. The startingDeadlineSeconds field will help us
	// schedule Jobs that are late.
	tooLate := false
	if vbsc.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*vbsc.Spec.StartingDeadlineSeconds) * time.Second).Before(time.Now())
	}
	if tooLate {
		log.Info("missed starting deadline for last run, sleeping till next")
		return scheduledResult, nil
	}

	// Check concurrency policy and skip this job if we have ForbidConcurrent set plus an active job
	if vbsc.Spec.ConcurrencyPolicy == planetscalev2.ForbidConcurrent && len(jobs.active) > 0 {
		log.Infof("concurrency policy blocks concurrent runs: skipping, number of active jobs: %d", len(jobs.active))
		return scheduledResult, nil
	}

	// Check concurrency policy to know if we should replace existing jobs
	if vbsc.Spec.ConcurrencyPolicy == planetscalev2.ReplaceConcurrent {
		for _, activeJob := range jobs.active {
			if err := r.client.Delete(ctx, activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.WithError(err).Error("unable to delete active job: %s", activeJob.Name)
				return ctrl.Result{}, err
			}
		}
	}

	// Now that the different policies are checked, we can create and apply our new job.
	job, err := r.createJob(ctx, &vbsc, missedRun)
	if err != nil {
		// Re-queuing here does not make sense as we have an error with the template and the user needs to fix it first.
		log.WithError(err).Error("unable to construct job from template")
		return ctrl.Result{}, err
	}
	if err = r.client.Create(ctx, job); err != nil {
		// if the job already exists it means another reconciling loop created the job since we last fetched
		// the list of jobs to create, we can safely return without failing.
		if apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, nil
		}
		// Simply re-queue here
		return resultBuilder.Error(err)
	}

	log.Infof("created new job: %s, next job scheduled in %s", job.Name, scheduledResult.RequeueAfter.String())
	return scheduledResult, nil
}

func getNextSchedule(vbsc planetscalev2.VitessBackupSchedule, now time.Time) (time.Time, time.Time, error) {
	sched, err := cron.ParseStandard(vbsc.Spec.Schedule)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("unparaseable schedule %q: %v", vbsc.Spec.Schedule, err)
	}

	// for optimization purposes, cheat a bit and start from our last observed run time
	// we could reconstitute this here, but there's not much point, since we've
	// just updated it.
	var earliestTime time.Time
	if vbsc.Status.LastScheduledTime != nil {
		earliestTime = vbsc.Status.LastScheduledTime.Time
	} else {
		earliestTime = vbsc.ObjectMeta.CreationTimestamp.Time
	}

	if vbsc.Spec.StartingDeadlineSeconds != nil {
		// controller is not going to schedule anything below this point
		schedulingDeadline := now.Add(-time.Second * time.Duration(*vbsc.Spec.StartingDeadlineSeconds))

		if schedulingDeadline.After(earliestTime) {
			earliestTime = schedulingDeadline
		}
	}

	// Next schedule is later, simply return the next scheduled time.
	if earliestTime.After(now) {
		return time.Time{}, sched.Next(now), nil
	}

	var lastMissed time.Time
	missedRuns := 0
	for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
		lastMissed = t
		missedRuns++

		// If we have too many missed jobs, just bail out as given on the clock lag, looping over the schedule might take forever.
		if missedRuns > vbsc.GetMissedRunsLimit() {
			log.Warnf("too many missed runs, skipping all previous runs and forwarding to the next scheduled time")
			return time.Time{}, sched.Next(now), nil
		}
	}

	return lastMissed, sched.Next(now), nil
}

func (r *ReconcileVitessBackupsSchedule) updateVitessBackupScheduleStatus(ctx context.Context, mostRecentTime *time.Time, vbsc planetscalev2.VitessBackupSchedule, activeJobs []*kbatch.Job) error {
	if mostRecentTime != nil {
		vbsc.Status.LastScheduledTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		vbsc.Status.LastScheduledTime = nil
	}

	vbsc.Status.Active = nil
	for _, activeJob := range activeJobs {
		jobRef, err := ref.GetReference(r.scheme, activeJob)
		if err != nil {
			log.WithError(err).Errorf("unable to make reference to active job: %s", jobRef.Name)
			continue
		}
		vbsc.Status.Active = append(vbsc.Status.Active, *jobRef)
	}

	if err := r.client.Status().Update(ctx, &vbsc); err != nil {
		log.WithError(err).Error("unable to update VitessBackupSchedule status")
		return err
	}
	return nil
}

// getJobsList fetches all existing Jobs in the cluster and return them by categories: active, failed or successful.
// It also returns at what time was the last job created, which is needed to update VitessBackupSchedule's status,
// and plan future jobs.
func (r *ReconcileVitessBackupsSchedule) getJobsList(ctx context.Context, req ctrl.Request, vbscName string) (jobsList, *time.Time, error) {
	var existingJobs kbatch.JobList

	err := r.client.List(ctx, &existingJobs, client.InNamespace(req.Namespace), client.MatchingLabels{planetscalev2.BackupScheduleLabel: vbscName})
	if err != nil && !apierrors.IsNotFound(err) {
		log.WithError(err).Error("unable to list Jobs in cluster")
		return jobsList{}, nil, err
	}

	var jobs jobsList

	var mostRecentTime *time.Time

	for i, job := range existingJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case kbatch.JobFailed:
			jobs.failed = append(jobs.failed, &existingJobs.Items[i])
		case kbatch.JobComplete:
			jobs.successful = append(jobs.successful, &existingJobs.Items[i])
		default:
			// Either: Suspended, FailureTarget or simply ongoing
			jobs.active = append(jobs.active, &existingJobs.Items[i])
		}

		scheduledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			log.WithError(err).Errorf("unable to parse schedule time for existing job, found: %s", job.Annotations[scheduledTimeAnnotation])
			continue
		}
		if scheduledTimeForJob != nil {
			if mostRecentTime == nil {
				mostRecentTime = scheduledTimeForJob
			} else if mostRecentTime.Before(*scheduledTimeForJob) {
				mostRecentTime = scheduledTimeForJob
			}
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
		if err := r.client.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
			log.WithError(err).Errorf("unable to delete old job: %s", job.Name)
		} else {
			log.Infof("deleted old job: %s", job.Name)
		}
	}
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

func (r *ReconcileVitessBackupsSchedule) createJob(ctx context.Context, vbsc *planetscalev2.VitessBackupSchedule, scheduledTime time.Time) (*kbatch.Job, error) {
	name := fmt.Sprintf("%s-%d", vbsc.Name, scheduledTime.Unix())

	meta := metav1.ObjectMeta{
		Labels: map[string]string{
			planetscalev2.BackupScheduleLabel: vbsc.Name,
		},
		Annotations: make(map[string]string),
		Name:        name,
		Namespace:   vbsc.Namespace,
	}
	for k, v := range vbsc.Annotations {
		meta.Annotations[k] = v
	}
	meta.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)

	for k, v := range vbsc.Labels {
		meta.Labels[k] = v
	}

	pod, err := r.createJobPod(ctx, vbsc, name)
	if err != nil {
		return nil, err
	}
	job := &kbatch.Job{
		ObjectMeta: meta,
		Spec: kbatch.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec:       pod,
			},
		},
	}

	if err := ctrl.SetControllerReference(vbsc, job, r.scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *ReconcileVitessBackupsSchedule) createJobPod(ctx context.Context, vbsc *planetscalev2.VitessBackupSchedule, name string) (pod corev1.PodSpec, err error) {
	vtctldServiceName, vtctldServicePort, err := r.getVtctldServiceName(ctx, vbsc)
	if err != nil {
		return corev1.PodSpec{}, err
	}
	vtctldServer := fmt.Sprintf("%s:%d", vtctldServiceName, vtctldServicePort)
	vtctldclientServerArg := fmt.Sprintf("--server=%s", vtctldServer)

	// It is fine to not have any default in the event there is no strategy as the CRD validation
	// ensures that there will be at least one item in this list. The YAML cannot be applied with
	// empty list of strategies.
	args := []string{"/bin/sh", "-c"}
	var cmd string

	for i, strategy := range vbsc.Spec.Strategy {
		if i > 0 {
			cmd = fmt.Sprintf("%s && ", cmd)
		}
		// At this point, strategy.Name is either BackupShard or BackupTablet, the validation
		// is made at the CRD level on the YAML directly.

		cmd = fmt.Sprintf("%s%s %s", cmd, vtctldclientPath, vtctldclientServerArg)

		// Add the vtctldclient command
		switch strategy.Name {
		case planetscalev2.BackupShard:
			cmd = fmt.Sprintf("%s BackupShard", cmd)
		case planetscalev2.BackupTablet:
			cmd = fmt.Sprintf("%s Backup", cmd)
		}

		// Add flags
		if strategy.AllowPrimary {
			cmd = fmt.Sprintf("%s --allow_primary=true", cmd)
		}

		if strategy.UpgradeSafe {
			cmd = fmt.Sprintf("%s --upgrade-safe=true", cmd)
		}

		// Add keyspace/shard or tablet alias
		switch strategy.Name {
		case planetscalev2.BackupShard:
			if strategy.KeyspaceShard == "" {
				return pod, fmt.Errorf("the KeyspaceShard field is missing from VitessBackupScheduleStrategy %s", planetscalev2.BackupShard)
			}
			cmd = fmt.Sprintf("%s %s", cmd, strategy.KeyspaceShard)
		case planetscalev2.BackupTablet:
			if strategy.TabletAlias == "" {
				return pod, fmt.Errorf("the TabletAlias field is missing from VitessBackupScheduleStrategy %s", planetscalev2.BackupTablet)
			}
			cmd = fmt.Sprintf("%s %s", cmd, strategy.TabletAlias)
		}
	}
	args = append(args, cmd)

	pod = corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:            name,
			Image:           vbsc.Spec.Image,
			ImagePullPolicy: vbsc.Spec.ImagePullPolicy,
			Resources:       vbsc.Spec.Resources,
			Args:            args,
		}},
		RestartPolicy: corev1.RestartPolicyOnFailure,
	}
	return pod, nil
}

func (r *ReconcileVitessBackupsSchedule) getVtctldServiceName(ctx context.Context, vbsc *planetscalev2.VitessBackupSchedule) (svcName string, svcPort int32, err error) {
	svcList := &corev1.ServiceList{}
	listOpts := &client.ListOptions{
		Namespace: vbsc.Namespace,
		LabelSelector: apilabels.Set{
			planetscalev2.ComponentLabel: planetscalev2.VtctldComponentName,
		}.AsSelector(),
	}
	if err = r.client.List(ctx, svcList, listOpts); err != nil {
		return "", 0, fmt.Errorf("unable to list vtctld service in %q: %v", vbsc.Namespace, err)
	}

	if len(svcList.Items) > 0 {
		service := svcList.Items[0]
		svcName = service.Name
		for _, port := range service.Spec.Ports {
			if port.Name == planetscalev2.DefaultGrpcPortName {
				svcPort = port.Port
				break
			}
		}
	}

	if svcName == "" || svcPort == 0 {
		return "", 0, fmt.Errorf("no vtctld service in %q found", vbsc.Namespace)
	}
	return svcName, svcPort, nil
}
