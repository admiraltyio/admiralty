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

package http

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
)

type LogsExecProvider struct {
	SourceClient  kubernetes.Interface
	TargetConfigs map[string]*rest.Config
	TargetClients map[string]kubernetes.Interface
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *LogsExecProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	targetName, delegatePodName, err := p.getTargetAndDelegatePodNames(ctx, namespace, podName)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get delegate pod name")
	}

	options := &v1.PodLogOptions{
		Container:  containerName,
		Timestamps: opts.Timestamps,
		Follow:     opts.Follow,
		Previous:   opts.Previous,
	}
	tailLine := int64(opts.Tail)
	if opts.Tail != 0 {
		options.TailLines = &tailLine
	}
	limitBytes := int64(opts.LimitBytes)
	if limitBytes != 0 {
		options.LimitBytes = &limitBytes
	}
	sinceSeconds := int64(opts.SinceSeconds)
	if sinceSeconds != 0 {
		options.SinceSeconds = &sinceSeconds
	}
	if !opts.SinceTime.IsZero() {
		options.SinceTime = &metav1.Time{Time: opts.SinceTime}
	}

	logs := p.TargetClients[targetName].CoreV1().Pods(namespace).GetLogs(delegatePodName, options)
	stream, err := logs.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get stream from logs request: %v", err)
	}
	return stream, nil
}

func (p *LogsExecProvider) getTargetAndDelegatePodNames(ctx context.Context, namespace string, podName string) (string, string, error) {
	proxyPod, err := p.SourceClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", "", errors.Wrap(err, "cannot get proxy pod")
	}
	targetName := proxypod.GetScheduledClusterName(proxyPod)
	if targetName == "" {
		return "", "", errors.Errorf("proxy pod isn't scheduled yet")
	}
	if _, ok := p.TargetClients[targetName]; !ok {
		return "", "", errors.Errorf("not a current target name")
	}
	l, err := p.TargetClients[targetName].CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: common.LabelKeyParentUID + "=" + string(proxyPod.UID)})
	if err != nil {
		return "", "", errors.Wrap(err, "cannot list delegate pod")
	}
	if len(l.Items) > 1 {
		return "", "", fmt.Errorf("more than one delegate pod")
	}
	if len(l.Items) < 1 {
		return "", "", fmt.Errorf("delegate pod not found")
	}
	delegatePodName := l.Items[0].Name
	return targetName, delegatePodName, nil
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *LogsExecProvider) RunInContainer(ctx context.Context, namespace, name, container string, cmd []string, attach api.AttachIO) error {
	defer func() {
		if attach.Stdout() != nil {
			attach.Stdout().Close()
		}
		if attach.Stderr() != nil {
			attach.Stderr().Close()
		}
	}()

	targetName, delegatePodName, err := p.getTargetAndDelegatePodNames(ctx, namespace, name)
	if err != nil {
		return errors.Wrap(err, "cannot get delegate pod name")
	}

	req := p.TargetClients[targetName].CoreV1().RESTClient().
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(delegatePodName).
		SubResource("exec").
		Timeout(0).
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdin:     attach.Stdin() != nil,
			Stdout:    attach.Stdout() != nil,
			Stderr:    attach.Stderr() != nil,
			TTY:       attach.TTY(),
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(p.TargetConfigs[targetName], "POST", req.URL())
	if err != nil {
		return fmt.Errorf("could not make remote command: %v", err)
	}

	ts := &termSize{attach: attach}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:             attach.Stdin(),
		Stdout:            attach.Stdout(),
		Stderr:            attach.Stderr(),
		Tty:               attach.TTY(),
		TerminalSizeQueue: ts,
	})
	if err != nil {
		return err
	}

	return nil
}

type termSize struct {
	attach api.AttachIO
}

func (t *termSize) Next() *remotecommand.TerminalSize {
	resize := <-t.attach.Resize()
	return &remotecommand.TerminalSize{
		Height: resize.Height,
		Width:  resize.Width,
	}
}
