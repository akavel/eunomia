/*
Copyright 2019 Kohl's Department Stores, Inc.

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

package gitopsconfig

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gitopsv1alpha1 "github.com/KohlsTechnology/eunomia/pkg/apis/eunomia/v1alpha1"
)

type statusUpdater struct {
	client client.Client
}

var _ cache.ResourceEventHandler = &statusUpdater{}

func (u *statusUpdater) OnAdd(newObj interface{})    { u.OnUpdate(nil, newObj) }
func (u *statusUpdater) OnDelete(oldObj interface{}) { u.OnUpdate(oldObj, nil) }

func (u *statusUpdater) OnUpdate(oldObj, newObj interface{}) {
	// Extract Job objects from arguments
	oldJob, ok := oldObj.(*batchv1.Job)
	if !ok && oldObj != nil {
		log.Error(nil, "non-Job object passed to statusUpdater", "oldObj", oldObj, "newObj", newObj)
		return
	}
	newJob, ok := newObj.(*batchv1.Job)
	if !ok && newObj != nil {
		log.Error(nil, "non-Job object passed to statusUpdater", "oldObj", oldObj, "newObj", newObj)
		return
	}

	// FIXME: check preconditions similar as in jobCompletionEmitter - but
	// allow incomplete jobs (Active > 0) - they will need to be marked as InProgress
	_ = oldJob
	if newJob == nil {
		return
	}

	// Check if this is a Job that's owned by GitOpsConfig.
	gitopsRef, err := findJobOwner(newJob, u.client)
	if err != nil {
		log.Error(err, "cannot find Job's owner", "job", newJob.Name)
		return
	}
	if gitopsRef == nil {
		// Got an event for a job not owned by GitOpsConfig - ignore it.
		return
	}

	// Calculate status
	status := gitopsv1alpha1.GitOpsConfigStatus{
		StartTime:      newJob.Status.StartTime,
		CompletionTime: newJob.Status.CompletionTime,
	}
	switch {
	case newJob.Status.Active > 0:
		status.State = "InProgress"
	case newJob.Status.Succeeded == 1:
		status.State = "Succeeded"
	case newJob.Status.Succeeded == 0 && newJob.Status.Failed > 0:
		status.State = "Failed"
	}

	// Update status
	gitops := &gitopsv1alpha1.GitOpsConfig{}
	err = u.client.Get(context.TODO(), types.NamespacedName{Name: gitopsRef.Name, Namespace: newJob.GetNamespace()}, gitops)
	if err != nil {
		log.Error(err, "cannot update GitOpsConfig")
		return
	}
	gitops.Status = status
	err = u.client.Status().Update(context.TODO(), gitops)
	if err != nil {
		// FIXME: should retry, especially because we may be rejected because of concurrent changes to GitOpsConfig object
		log.Error(err, "Failed to update status", "GitOpsConfig", gitops.Name, "job", newJob.Name)
		return
	}
}
