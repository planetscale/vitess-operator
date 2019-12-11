package vitesscluster

import (
	"context"
	"testing"

	"planetscale.dev/vitess-operator/test/integration/framework"
)

func TestMain(m *testing.M) {
	framework.TestMain(m.Run)
}

func TestVitesCluster(t *testing.T) {
	ctx := context.Background()

	f := framework.NewFixture(t)
	defer f.TearDown(ctx)

	// TODO: Test something. This is just a skeleton.
}
