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

package v1

// SetDefaults_ProxyArgs sets the default parameters for the Proxy plugin
func SetDefaults_ProxyArgs(args *ProxyArgs) {
	if args.FilterWaitDurationSeconds == nil {
		defaultTime := int32(30)
		args.FilterWaitDurationSeconds = &defaultTime
	}
}

// SetDefaults_CandidateArgs sets the default parameters for the Candidate plugin
func SetDefaults_CandidateArgs(args *CandidateArgs) {
	if args.PreBindWaitDurationSeconds == nil {
		defaultTime := int32(30)
		args.PreBindWaitDurationSeconds = &defaultTime
	}
}
