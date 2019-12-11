package vitesscluster

import (
	"testing"

	"planetscale.dev/vitess-operator/test/integration/framework"
)

func TestMain(m *testing.M) {
	framework.TestMain(m.Run)
}

func TestVitesCluster(t *testing.T) {
	f := framework.NewFixture(t)
	defer f.TearDown()

	// TODO: Test something. This is just a skeleton.
}
