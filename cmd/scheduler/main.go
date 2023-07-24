/*
 * Copyright 2023 The Multicluster-Scheduler Authors.
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

package main

import (
	"os"

	"admiralty.io/multicluster-scheduler/pkg/scheduler_plugins/candidate"
	"admiralty.io/multicluster-scheduler/pkg/scheduler_plugins/proxy"
	"k8s.io/component-base/cli"
	scheduler "k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {
	// BEWARE candidate and proxy must run in different processes, because a scheduler only processes one pod at a time
	// and proxy waits on candidates in filter plugin

	command := scheduler.NewSchedulerCommand(
		scheduler.WithPlugin(candidate.Name, candidate.New),
		scheduler.WithPlugin(proxy.Name, proxy.New))

	code := cli.Run(command)
	os.Exit(code)
}
