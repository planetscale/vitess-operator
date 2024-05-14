package vitessbackupschedule

import (
	"context"
	"flag"
	"fmt"

	// "fmt"
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
)

// watchResources should contain all the resource types that this controller creates.
var watchResources = []client.Object{
	&kbatch.Job{},
}

var log = logrus.WithField("controller", "VitessBackupSchedules")

var _ reconcile.Reconciler = &ReconcileVitessBackupsSchedule{}

// ReconcileVitessBackupsSchedule reconciles a CronJob object
type ReconcileVitessBackupsSchedule struct {
	client     client.Client
	scheme     *runtime.Scheme
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

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

	// Watch for changes to primary resource VitessCluster
	if err := c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessBackupSchedule{}), &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessCluster.
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

var (
	scheduledTimeAnnotation = "planetscale.com/backup-scheduled-at"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReconcileVitessBackupsSchedule) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	resultBuilder := &results.Builder{}

	// Load CronJob by name:
	var vbsc planetscalev2.VitessBackupSchedule
	if err := r.client.Get(ctx, req.NamespacedName, &vbsc); err != nil {
		log.Error(err, " unable to fetch CronJob")
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return resultBuilder.Result()
		}
		// Error reading the object - requeue the request.
		return resultBuilder.Error(err)
	}

	// List all active jobs:
	var childJobs kbatch.JobList
	if err := r.client.List(ctx, &childJobs, client.InNamespace(req.Namespace)); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, " unable to list child Jobs")
		return ctrl.Result{}, err
	}

	// find the active list of jobs
	var activeJobs []*kbatch.Job
	var successfulJobs []*kbatch.Job
	var failedJobs []*kbatch.Job

	// find the last run, so we can update the status
	var mostRecentTime *time.Time

	for i, job := range childJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "": // ongoing
			activeJobs = append(activeJobs, &childJobs.Items[i])
		case kbatch.JobFailed:
			failedJobs = append(failedJobs, &childJobs.Items[i])
		case kbatch.JobComplete:
			successfulJobs = append(successfulJobs, &childJobs.Items[i])
		}

		// We'll store the launch time in annotation, so we'll reconstitute that from the active jobs themselves.
		scheduledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			log.Error(err, "unable to parse schedule time for child job", "job", &job)
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

	if mostRecentTime != nil {
		vbsc.Status.LastScheduledTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		vbsc.Status.LastScheduledTime = nil
	}

	vbsc.Status.Active = nil
	for _, activeJob := range activeJobs {
		jobRef, err := ref.GetReference(r.scheme, activeJob)
		if err != nil {
			log.Error(err, "unable to make reference to active job", "job", activeJob)
			continue
		}
		vbsc.Status.Active = append(vbsc.Status.Active, *jobRef)
	}

	log.Info("job count", "active jobs", len(activeJobs), "successful jobs", len(successfulJobs), "failed jobs", len(failedJobs))

	if err := r.client.Status().Update(ctx, &vbsc); err != nil {
		log.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, err
	}

	// Clean up old jobs according to the history limit
	if vbsc.Spec.FailedJobsHistoryLimit != nil {
		sort.Slice(failedJobs, func(i, j int) bool {
			if failedJobs[i].Status.StartTime == nil {
				return failedJobs[j].Status.StartTime != nil
			}
			return failedJobs[i].Status.StartTime.Before(failedJobs[j].Status.StartTime)
		})

		for i, job := range failedJobs {
			if int32(i) >= int32(len(failedJobs))-*vbsc.Spec.FailedJobsHistoryLimit {
				break
			}
			if err := r.client.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete old failed job", "job", job)
			} else {
				log.Info("deleted old failed job", "job", job)
			}
		}
	}

	if vbsc.Spec.SuccessfulJobsHistoryLimit != nil {
		sort.Slice(successfulJobs, func(i, j int) bool {
			if successfulJobs[i].Status.StartTime == nil {
				return successfulJobs[j].Status.StartTime != nil
			}
			return successfulJobs[j].Status.StartTime.Before(successfulJobs[j].Status.StartTime)
		})

		for i, job := range successfulJobs {
			if int32(i) >= int32(len(successfulJobs))-*vbsc.Spec.SuccessfulJobsHistoryLimit {
				break
			}
			if err := r.client.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
				log.Error(err, "unable to delete old successful job", "job", job)
			} else {
				log.Info("deleted old successful job", "job", job)
			}
		}
	}

	// Check if we’re suspended
	if vbsc.Spec.Suspend != nil && *vbsc.Spec.Suspend {
		log.Info("cronjob suspended, skipping")
		return ctrl.Result{}, nil
	}

	getNextSchedule := func(cronJob *planetscalev2.VitessBackupSchedule, now time.Time) (lastMissed time.Time, next time.Time, err error) {
		sched, err := cron.ParseStandard(cronJob.Spec.Schedule)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("Unparaseable schedule %q: %v", cronJob.Spec.Schedule, err)
		}

		// for optimization purposes, cheat a bit and start from our last observed run time
		// we could reconstitute this here, but there's not much point, since we've
		// just updated it.
		var earliestTime time.Time
		if cronJob.Status.LastScheduledTime != nil {
			earliestTime = cronJob.Status.LastScheduledTime.Time
		} else {
			earliestTime = cronJob.ObjectMeta.CreationTimestamp.Time
		}

		if vbsc.Spec.StartingDeadlineSeconds != nil {
			// controller is not going to schedule anything below this point
			schedulingDeadline := now.Add(-time.Second * time.Duration(*vbsc.Spec.StartingDeadlineSeconds))

			if schedulingDeadline.After(earliestTime) {
				earliestTime = schedulingDeadline
			}
		}

		if earliestTime.After(now) {
			return time.Time{}, sched.Next(now), nil
		}

		starts := 0
		for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
			lastMissed = t

			// An object might miss several starts. For example, if
			// controller gets wedged on Friday at 5:01pm when everyone has
			// gone home, and someone comes in on Tuesday AM and discovers
			// the problem and restarts the controller, then all the hourly
			// jobs, more than 80 of them for one hourly scheduledJob, should
			// all start running with no further intervention (if the scheduledJob
			// allows concurrency and late starts).
			//
			// However, if there is a bug somewhere, or incorrect clock
			// on controller's server or apiservers (for setting creationTimestamp)
			// then there could be so many missed start times (it could be off
			// by decades or more), that it would eat up all the CPU and memory
			// of this controller. In that case, we want to not try to list
			// all the missed start times.
			starts++
			if starts > 100 {
				// We can't get the most recent times, so just return an empty slice
				return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100). Set or decrease .spec.StartingDeadlineSeconds or check clock skew.")
			}
		}

		return lastMissed, sched.Next(now), nil
	}

	// Figure out the nex times that we need to create jobs at (or anything we missed)
	missedRun, nextRun, err := getNextSchedule(&vbsc, time.Now())
	if err != nil {
		log.Error(err, "unable to figure out CronJob schedule")
		// We don't really care about requeuing until we get an update that fixes the schedule, so don't return an error
		return ctrl.Result{}, nil
	}

	// We'll prepare our eventual request to requeue until the next job, and then figure out if we actually need to run
	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(time.Now())}
	log.Info("now", time.Now(), "next run", nextRun)

	// Run a new job if it's on schedule, not past the deadline, and not blocked by our concurrency policy
	if missedRun.IsZero() {
		log.Info("no upcoming scheduled times, sleeping until next")
		return scheduledResult, nil
	}

	// make sure we're not too late to start the run
	log.Info("current run", missedRun)
	tooLate := false
	if vbsc.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*vbsc.Spec.StartingDeadlineSeconds) * time.Second).Before(time.Now())
	}
	if tooLate {
		log.Info("missed starting deadline for last run, sleeping till next")
		return scheduledResult, nil
	}

	// figure out how to run this job -- concurrency policy might forbid us from running multiple at the same time...
	if vbsc.Spec.ConcurrencyPolicy == planetscalev2.ForbidConcurrent && len(activeJobs) > 0 {
		log.Info("concurrency policy blocks concurrent runs, skipping", "num active", len(activeJobs))
		return scheduledResult, nil
	}

	// ...or instruct us to replace existing ones...
	if vbsc.Spec.ConcurrencyPolicy == planetscalev2.ReplaceConcurrent {
		for _, activeJob := range activeJobs {
			// we don't care if the job was already deleted
			if err := r.client.Delete(ctx, activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete active job", "job", activeJob)
				return ctrl.Result{}, err
			}
		}
	}

	// actually make the job...
	job, err := r.createJob(ctx, &vbsc, missedRun)
	if err != nil {
		log.Error(err, "unable to construct job from template")
		return scheduledResult, nil
	}

	// ...and create it on the cluster
	if err := r.client.Create(ctx, job); err != nil {
		log.Error(err, "unable to create Job for CronJob", "job", job)
		return ctrl.Result{}, err
	}

	log.Info("created Job for CronJob run", "job", job)

	return scheduledResult, nil
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
	shortName := fmt.Sprintf("%s-%s", vbsc.Name, vbsc.Spec.Name)
	name := fmt.Sprintf("%s-%d", shortName, scheduledTime.Unix())

	meta := metav1.ObjectMeta{
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		Name:        name,
		Namespace:   vbsc.Namespace,
	}
	pod, err := r.createJobPod(ctx, vbsc, shortName)
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
	for k, v := range vbsc.Annotations {
		job.Annotations[k] = v
	}
	job.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)

	for k, v := range vbsc.Labels {
		job.Labels[k] = v
	}

	if err := ctrl.SetControllerReference(vbsc, job, r.scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *ReconcileVitessBackupsSchedule) createJobPod(ctx context.Context, vbsc *planetscalev2.VitessBackupSchedule, shortName string) (pod corev1.PodSpec, err error) {
	vtctldServiceName, vtctldServicePort, err := r.getVtctldServiceName(ctx, vbsc)
	if err != nil {
		return corev1.PodSpec{}, err
	}
	vtctldServer := fmt.Sprintf("%s:%d", vtctldServiceName, vtctldServicePort)

	var args []string
	if vbsc.Spec.Strategy.BackupShard != nil {
		// vtctldclient --server=<vtctld_host>:<vtctld_port> BackupShard [--allow_primary=false] [--upgrade-safe=false] <keyspace/shard>

		args = append(args, vtctldclientPath, fmt.Sprintf("--server=%s", vtctldServer), "BackupShard")

		if vbsc.Spec.Strategy.BackupShard.AllowPrimary {
			args = append(args, "--allow_primary=true")
		}

		if vbsc.Spec.Strategy.BackupShard.UpgradeSafe {
			args = append(args, "--upgrade-safe=true")
		}

		args = append(args, fmt.Sprintf("%s/%s", vbsc.Spec.Strategy.BackupShard.Keyspace, vbsc.Spec.Strategy.BackupShard.Shard))
	} else if vbsc.Spec.Strategy.BackupTablet != nil {

	} else {

	}

	pod = corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:            shortName,
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
