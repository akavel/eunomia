package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	eventv1beta1 "k8s.io/api/events/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/KohlsTechnology/eunomia/pkg/apis"
	gitopsv1alpha1 "github.com/KohlsTechnology/eunomia/pkg/apis/eunomia/v1alpha1"
	"github.com/KohlsTechnology/eunomia/test"
)

func TestStatus_Succeeded(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	defer DumpJobsLogsOnError(t, framework.Global, namespace)
	err = framework.AddToFrameworkScheme(apis.AddToScheme, &gitopsv1alpha1.GitOpsConfigList{})
	if err != nil {
		t.Fatal(err)
	}

	eunomiaURI, found := os.LookupEnv("EUNOMIA_URI")
	if !found {
		eunomiaURI = "https://github.com/kohlstechnology/eunomia"
	}
	eunomiaRef, found := os.LookupEnv("EUNOMIA_REF")
	if !found {
		eunomiaRef = "master"
	}

	// Step 1: register a status monitor/watcher

	statuses := make(chan *gitopsv1alpha1.GitOpsConfigStatus, 5)
	closer, err := watchStatus(framework.Global.KubeClient, statuses, namespace, "gitops-status-hello-success", 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	// Step 2: create a simple CR with a single Pod

	gitops := &gitopsv1alpha1.GitOpsConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitOpsConfig",
			APIVersion: "eunomia.kohls.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitops-events-hello-success",
			Namespace: namespace,
		},
		Spec: gitopsv1alpha1.GitOpsConfigSpec{
			TemplateSource: gitopsv1alpha1.GitConfig{
				URI:        eunomiaURI,
				Ref:        eunomiaRef,
				ContextDir: "test/e2e/testdata/events/test-a",
			},
			ParameterSource: gitopsv1alpha1.GitConfig{
				URI:        eunomiaURI,
				Ref:        eunomiaRef,
				ContextDir: "test/e2e/testdata/empty-yaml",
			},
			Triggers: []gitopsv1alpha1.GitOpsTrigger{
				{Type: "Change"},
			},
			TemplateProcessorImage: "quay.io/kohlstechnology/eunomia-base:dev",
			ResourceHandlingMode:   "Apply",
			ResourceDeletionMode:   "Delete",
			ServiceAccountRef:      "eunomia-operator",
		},
	}
	gitops.Annotations = map[string]string{"gitopsconfig.eunomia.kohls.io/initialized": "true"}

	err = framework.Global.Client.Create(context.TODO(), gitops, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}

	err = WaitForPodWithImage(t, framework.Global, namespace, "hello-events-test-a", "hello-app:1.0", retryInterval, timeout)
	if err != nil {
		t.Error(err)
	}

	// Step 3: Verify events

	// Verify there was an event emitted mentioning that Job finished successfully
	select {
	case event := <-events:
		if event.Reason != "JobSuccessful" ||
			event.DeprecatedSource.Component != "gitopsconfig-controller" {
			t.Errorf("got bad event: %v", event)
		}
	case <-time.After(10 * time.Second):
		t.Errorf("timeout waiting for JobSuccessful event")
	}
	// Verify there are no more events
	select {
	case event := <-events:
		t.Errorf("unexpected extra event: %v", event)
	default:
		// ok, no events
	}
}

// TestJobEvents_PeriodicJobSuccess verifies that a JobSuccessful event is
// emitted by eunomia for a Periodic GitOpsConfig.
func TestJobEvents_PeriodicJobSuccess(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	defer DumpJobsLogsOnError(t, framework.Global, namespace)
	err = framework.AddToFrameworkScheme(apis.AddToScheme, &gitopsv1alpha1.GitOpsConfigList{})
	if err != nil {
		t.Fatal(err)
	}

	eunomiaURI, found := os.LookupEnv("EUNOMIA_URI")
	if !found {
		eunomiaURI = "https://github.com/kohlstechnology/eunomia"
	}
	eunomiaRef, found := os.LookupEnv("EUNOMIA_REF")
	if !found {
		eunomiaRef = "master"
	}

	// Step 1: register an event monitor/watcher

	events := make(chan *eventv1beta1.Event, 5)
	closer, err := test.WatchEvents(framework.Global.KubeClient, events, namespace, "gitops-events-periodic-success", 180*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	// Step 2: create a simple CR with a single Pod

	gitops := &gitopsv1alpha1.GitOpsConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitOpsConfig",
			APIVersion: "eunomia.kohls.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitops-events-periodic-success",
			Namespace: namespace,
		},
		Spec: gitopsv1alpha1.GitOpsConfigSpec{
			TemplateSource: gitopsv1alpha1.GitConfig{
				URI:        eunomiaURI,
				Ref:        eunomiaRef,
				ContextDir: "test/e2e/testdata/events/test-b",
			},
			ParameterSource: gitopsv1alpha1.GitConfig{
				URI:        eunomiaURI,
				Ref:        eunomiaRef,
				ContextDir: "test/e2e/testdata/empty-yaml",
			},
			Triggers: []gitopsv1alpha1.GitOpsTrigger{
				{
					Type: "Periodic",
					Cron: "*/1 * * * *",
				},
			},
			TemplateProcessorImage: "quay.io/kohlstechnology/eunomia-base:dev",
			ResourceHandlingMode:   "Apply",
			ResourceDeletionMode:   "Delete",
			ServiceAccountRef:      "eunomia-operator",
		},
	}
	gitops.Annotations = map[string]string{"gitopsconfig.eunomia.kohls.io/initialized": "true"}

	err = framework.Global.Client.Create(context.TODO(), gitops, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}

	err = WaitForPodWithImage(t, framework.Global, namespace, "hello-events-test-b", "hello-app:1.0", retryInterval, 2*time.Minute)
	if err != nil {
		t.Error(err)
	}

	// Step 3: Verify events

	// Verify there was an event emitted mentioning that Job finished successfully
	select {
	case event := <-events:
		if event.Reason != "JobSuccessful" ||
			event.DeprecatedSource.Component != "gitopsconfig-controller" {
			t.Errorf("got bad event: %v", event)
		}
	case <-time.After(10 * time.Second):
		t.Errorf("timeout waiting for JobSuccessful event")
	}
}

// TestJobEvents_JobFailed verifies that a JobFailed event is emitted by
// eunomia if a Job implementing GitOpsConfig fails. In particular, a bad URI
// is provided in the GitOpsConfig, to ensure that the Job will fail.
func TestJobEvents_JobFailed(t *testing.T) {
	if testing.Short() {
		// FIXME: as of writing this test, "backoffLimit" in job.yaml is set to 4,
		// which means we need to wait until 5 Pod retries fail, eventually
		// triggering a Job failure; the back-off time between the runs is
		// unfortunately exponential and non-configurable, which makes this test
		// awfully long. Try to at least make it possible to run in parallel with
		// other tests.
		t.Skip("This test currently takes minutes to run, because of exponential backoff in kubernetes")
	}

	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	defer DumpJobsLogsOnError(t, framework.Global, namespace)
	err = framework.AddToFrameworkScheme(apis.AddToScheme, &gitopsv1alpha1.GitOpsConfigList{})
	if err != nil {
		t.Fatal(err)
	}

	eunomiaURI, found := os.LookupEnv("EUNOMIA_URI")
	if !found {
		eunomiaURI = "https://github.com/kohlstechnology/eunomia"
	}
	eunomiaRef, found := os.LookupEnv("EUNOMIA_REF")
	if !found {
		eunomiaRef = "master"
	}

	// Step 1: register an event monitor/watcher

	events := make(chan *eventv1beta1.Event, 5)
	closer, err := test.WatchEvents(framework.Global.KubeClient, events, namespace, "gitops-events-hello-failed", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	// Step 2: create a CR with an invalid URI

	gitops := &gitopsv1alpha1.GitOpsConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitOpsConfig",
			APIVersion: "eunomia.kohls.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitops-events-hello-failed",
			Namespace: namespace,
		},
		Spec: gitopsv1alpha1.GitOpsConfigSpec{
			TemplateSource: gitopsv1alpha1.GitConfig{
				URI:        "https://INVALID!!!",
				Ref:        eunomiaRef,
				ContextDir: "URI is already invalid so this value should be irrelevant",
			},
			ParameterSource: gitopsv1alpha1.GitConfig{
				URI:        eunomiaURI,
				Ref:        eunomiaRef,
				ContextDir: "test/e2e/testdata/empty-yaml",
			},
			Triggers: []gitopsv1alpha1.GitOpsTrigger{
				{Type: "Change"},
			},
			TemplateProcessorImage: "quay.io/kohlstechnology/eunomia-base:dev",
			ResourceHandlingMode:   "Apply",
			ResourceDeletionMode:   "Delete",
			ServiceAccountRef:      "eunomia-operator",
		},
	}
	gitops.Annotations = map[string]string{"gitopsconfig.eunomia.kohls.io/initialized": "true"}

	err = framework.Global.Client.Create(context.TODO(), gitops, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}

	// Step 3: Verify events

	// Verify there was an event emitted mentioning that Job failed
	select {
	case event := <-events:
		if event.Reason != "JobFailed" ||
			event.DeprecatedSource.Component != "gitopsconfig-controller" {
			t.Errorf("got bad event: %v", event)
		}
	case <-time.After(3 * time.Minute):
		t.Errorf("timeout waiting for JobFailed event")
	}
}

func watchStatus(client kubernetes.Interface, statuses chan<- gitopsv1alpha1.GitOpsConfigStatus, namespace, name string, timeout time.Duration) (closer func(), err error) {
	timeoutSeconds := int64(timeout / time.Second)
	opts := &metav1.ListOptions{
		TimeoutSeconds: &timeoutSeconds,
	}
	watcher, err := client.Discovery().
		RESTClient().
		Get().
		Namespace(namespace).
		Resource("gitopsconfigs").
		Name(name).
		VersionedParams(opts, scheme.ParameterCodec).
		Timeout(timeout).Watch()
	if err != nil {
		return nil, err
	}
	go func() { // based on: https://stackoverflow.com/a/54930836
		ch := watcher.ResultChan()
		for {
			select {
			case change, ok := <-ch:
				if !ok {
					// Channel closed, finish watching.
					close(statuses)
					return
				}
				if change.Type != watch.Added {
					continue
				}
				gitops, ok := change.Object.(*gitopsv1alpha1.GitOpsConfig)
				if !ok || gitops.Name != name {
					fmt.Fprintf(os.Stderr, "GOT: %T\n", change.Object)
					continue
				}
				statuses <- gitops.Status
			}
		}
	}()
	return func() { watcher.Stop() }, nil
}
