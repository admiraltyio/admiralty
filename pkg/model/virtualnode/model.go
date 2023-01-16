/*
 * Copyright 2022 The Multicluster-Scheduler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package virtualnode

import (
	"admiralty.io/multicluster-scheduler/pkg/common"
)

func BaseLabels(targetNamespace, targetName string) map[string]string {
	l := map[string]string{
		"type": "virtual-kubelet",
		common.LabelAndTaintKeyVirtualKubeletProvider:             common.VirtualKubeletProviderName,
		"kubernetes.io/role":                                      "cluster",
		"alpha.service-controller.kubernetes.io/exclude-balancer": "true",
		"node.kubernetes.io/exclude-from-external-load-balancers": "true",
	}
	if targetNamespace == "" {
		l[common.LabelKeyClusterTargetName] = targetName
	} else {
		l[common.LabelKeyTargetNamespace] = targetNamespace
		l[common.LabelKeyTargetName] = targetName
	}
	return l
}
