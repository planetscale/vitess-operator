package vitessbackup

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ScheduleName(clusterName string) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, "backupschedule")
}

func NewVitessBackupSchedule(key client.ObjectKey, vt *planetscalev2.VitessCluster, labels map[string]string) *planetscalev2.VitessBackupSchedule {
	if vt.Spec.Backup == nil || vt.Spec.Backup.Schedule.Schedule == "" {
		return nil
	}

	return &planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			// We simply re-apply the same template that was written by the user.
			VitessBackupScheduleTemplate: vt.Spec.Backup.Schedule,

			// To take backups we only care about having the vtctldclient installed in the container.
			// For this reason, we re-use the vtctld Docker image and the same image pull policy.
			Image:           vt.Spec.Images.Vtctld,
			ImagePullPolicy: vt.Spec.ImagePullPolicies.Vtctld,
		},
	}
}
