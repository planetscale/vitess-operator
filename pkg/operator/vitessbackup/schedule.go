package vitessbackup

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ScheduleName(clusterName string) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, "backupschedule")
}

func NewVitessBackupSchedule(key client.ObjectKey, vt *planetscalev2.VitessCluster, labels map[string]string) runtime.Object {
	if vt.Spec.Backup == nil || vt.Spec.Backup.Schedule.Schedule == "" {
		return nil
	}

	schedule := vt.Spec.Backup.Schedule
	return &planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			VitessBackupScheduleTemplate: schedule,
		},
	}
}
