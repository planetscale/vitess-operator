module planetscale.dev/vitess-operator

go 1.12

require (
	contrib.go.opencensus.io/exporter/ocagent v0.4.12 // indirect
	github.com/Azure/go-autorest v11.7.1+incompatible // indirect
	github.com/ahmetb/gen-crd-api-reference-docs v0.1.5-0.20190629210212-52e137b8d003
	github.com/aws/aws-sdk-go v1.20.4 // indirect
	github.com/coreos/etcd v3.3.12+incompatible // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/go-openapi/spec v0.19.0
	github.com/golang/snappy v0.0.1 // indirect
	github.com/gophercloud/gophercloud v0.0.0-20190410012400-2c55d17f707c // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/hashicorp/consul v1.4.4 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.0 // indirect
	github.com/hashicorp/serf v0.8.3 // indirect
	github.com/klauspost/compress v1.7.5 // indirect
	github.com/klauspost/cpuid v1.2.1 // indirect
	github.com/klauspost/pgzip v1.2.1 // indirect
	github.com/operator-framework/operator-sdk v0.10.0
	github.com/prometheus/client_golang v1.1.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190612125737-db0771252981
	k8s.io/apimachinery v0.0.0-20190612125636-6a5db36e93ad
	k8s.io/apiserver v0.0.0-20190228174905-79427f02047f // indirect
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v0.3.1
	k8s.io/kube-openapi v0.0.0-20190320154901-5e45bb682580
	k8s.io/kubernetes v1.13.4
	k8s.io/utils v0.0.0-20190308190857-21c4ce38f2a7
	sigs.k8s.io/controller-runtime v0.1.12
	vitess.io/vitess v0.0.0-20191218033018-5644314df177
)

// ****************************
// BEGIN GENERATED OPERATOR-SDK
// ****************************
// Don't edit this section except by updating it to the values generated by
// operator-sdk when updating to a new operator-sdk version.
// ****************************
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
)

replace (
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.29.0
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.0.0-20190424153033-d3245f150225
	k8s.io/kube-state-metrics => k8s.io/kube-state-metrics v1.6.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.1.11-0.20190411181648-9d55346c2bde
)

replace github.com/operator-framework/operator-sdk => github.com/operator-framework/operator-sdk v0.10.0

// ****************************
// END GENERATED OPERATOR-SDK
// ****************************

// The git.apache.org server doesn't load. Go directly to GitHub.
replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0
