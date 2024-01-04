/*
Copyright 2019 PlanetScale Inc.

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

package vttablet

import (
	"fmt"
	"strings"

	"planetscale.dev/vitess-operator/pkg/operator/lazy"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	extraMyCnf.Add(func(s lazy.Spec) []string {
		return defaultExtraMyCnf
	})

	tabletEnvVars.Add(func(s lazy.Spec) []corev1.EnvVar {
		spec := s.(*Spec)
		return []corev1.EnvVar{
			{Name: "VTROOT", Value: vtRootPath},
			{Name: "VTDATAROOT", Value: vtDataRootPath},
			{Name: "VT_MYSQL_ROOT", Value: vtMysqlRootPath},
			{Name: "MYSQL_FLAVOR", Value: spec.Images.Mysqld.Flavor()},
			{
				Name:  "EXTRA_MY_CNF",
				Value: strings.Join(extraMyCnf.Get(spec), ":"),
			},
		}
	})

	mysqldExporterEnvVars.Add(func(s lazy.Spec) []corev1.EnvVar {
		return []corev1.EnvVar{
			{
				Name:  "DATA_SOURCE_NAME",
				Value: fmt.Sprintf("%s@unix(%s)/", mysqldExporterUser, mysqlSocketPath),
			},
		}
	})
}
