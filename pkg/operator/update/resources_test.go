package update

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

var (
	testKeyspaceTemplateEqual = planetscalev2.VitessKeyspaceTemplate{
		Name: "test",
		Partitionings: []planetscalev2.VitessKeyspacePartitioning{
			{
				Equal: &planetscalev2.VitessKeyspaceEqualPartitioning{
					Parts: 2,
					ShardTemplate: planetscalev2.VitessShardTemplate{
						TabletPools:              []planetscalev2.VitessShardTabletPool{
							{
								DataVolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
									Resources:        corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.Quantity{
												i: 30,
											},
										},
									},
								},
							},
						},
						DatabaseInitScriptSecret: planetscalev2.SecretSource{},
					},
				},
				Custom: nil,
			},
		},
		TurndownPolicy: planetscalev2.VitessKeyspaceTurndownPolicyImmediate,
	}

	testKeyspaceTemplateEqual2 =
)

func TestKeyspaceDiskSize(t *testing.T) {

}