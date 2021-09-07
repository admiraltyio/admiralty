/*
 * Copyright 2021 The Multicluster-Scheduler Authors.
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

package leaderelection

import (
	"context"
	"os"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func Run(ctx context.Context, ns, name string, k kubernetes.Interface, onStartedLeading func(ctx context.Context)) {
	id := os.Getenv("POD_NAME")
	if id == "" {
		// while remote-debugging, we may not have a pod name
		id = strconv.Itoa(os.Getpid())
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Client: k.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity:      id,
				EventRecorder: nil, // TODO...
			},
		},
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onStartedLeading,
			OnStoppedLeading: func() {
				// TODO log
			},
		},
		WatchDog:        nil, // TODO...
		ReleaseOnCancel: true,
		Name:            name,
	})
}
