/*
 * Copyright 2020 The Multicluster-Scheduler Authors.
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

package agent

import (
	"flag"
	"io/ioutil"
	"log"

	configv1alpha2 "admiralty.io/multicluster-scheduler/pkg/apis/config/v1alpha2"
	"sigs.k8s.io/yaml"
)

func New() *configv1alpha2.Agent {
	cfgPath := flag.String("config", "/etc/admiralty/config", "")
	flag.Parse()
	s, err := ioutil.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("cannot open agent configuration: %v", err)
	}
	raw := &configv1alpha2.Agent{}
	if err := yaml.Unmarshal(s, raw); err != nil {
		log.Fatalf("cannot unmarshal agent configuration: %v", err)
	}
	return raw
}
