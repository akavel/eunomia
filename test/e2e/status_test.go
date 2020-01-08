package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/KohlsTechnology/eunomia/pkg/apis"
	gitopsv1alpha1 "github.com/KohlsTechnology/eunomia/pkg/apis/eunomia/v1alpha1"
)

func TestStatus_Succeeded(t *testing.T) {
	fmt.Println("START JobFailed")
	defer fmt.Println("END JobFailed")

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

	// Step 1: create a simple CR with a single Pod

	gitops := &gitopsv1alpha1.GitOpsConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitOpsConfig",
			APIVersion: "eunomia.kohls.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitops-status-hello-success",
			Namespace: namespace,
		},
		Spec: gitopsv1alpha1.GitOpsConfigSpec{
			TemplateSource: gitopsv1alpha1.GitConfig{
				URI:        eunomiaURI,
				Ref:        eunomiaRef,
				ContextDir: "test/e2e/testdata/status/test-a",
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

	// Step 2: watch Status till Succeeded & verify Status fields

	err = wait.Poll(retryInterval, 25*time.Second, func() (done bool, err error) {
		fresh := gitopsv1alpha1.GitOpsConfig{}
		err = framework.Global.Client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: gitops.Name}, &fresh)
		if err != nil {
			return false, err
		}
		switch fresh.Status.State {
		case "InProgress":
			// TODO: check that fresh.Status.StartTime is in the past, but after whole test started
			if fresh.Status.CompletionTime != nil {
				t.Errorf("want CompletionTime==nil, got: %#v", fresh.Status)
			}
			return false, nil
		case "Succeeded":
			if fresh.Status.CompletionTime == nil {
				t.Errorf("CompletionTime==nil in: %#v", fresh.Status)
			}
			return true, nil
		default:
			t.Errorf("Unexpected State: %#v", fresh.Status)
			return false, nil
		}
	})
	if err != nil {
		t.Error(err)
	}

	// Step 3: verify that the pod exists

	pod, err := GetPod(namespace, "hello-status-test-a", "hello-app:1.0", framework.Global.KubeClient)
	if err != nil {
		t.Fatal(err)
	}
	if pod == nil || pod.Status.Phase != "Running" {
		t.Fatalf("unexpected state of Pod: %v", pod)
	}
}

// // TestJobEvents_PeriodicJobSuccess verifies that a JobSuccessful event is
// // emitted by eunomia for a Periodic GitOpsConfig.
// func TestJobEvents_PeriodicJobSuccess(t *testing.T) {
// 	ctx := framework.NewTestCtx(t)
// 	defer ctx.Cleanup()

// 	namespace, err := ctx.GetNamespace()
// 	if err != nil {
// 		t.Fatalf("could not get namespace: %v", err)
// 	}
// 	defer DumpJobsLogsOnError(t, framework.Global, namespace)
// 	err = framework.AddToFrameworkScheme(apis.AddToScheme, &gitopsv1alpha1.GitOpsConfigList{})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	eunomiaURI, found := os.LookupEnv("EUNOMIA_URI")
// 	if !found {
// 		eunomiaURI = "https://github.com/kohlstechnology/eunomia"
// 	}
// 	eunomiaRef, found := os.LookupEnv("EUNOMIA_REF")
// 	if !found {
// 		eunomiaRef = "master"
// 	}

// 	// Step 1: register an event monitor/watcher

// 	events := make(chan *eventv1beta1.Event, 5)
// 	closer, err := test.WatchEvents(framework.Global.KubeClient, events, namespace, "gitops-events-periodic-success", 180*time.Second)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer closer()

// 	// Step 2: create a simple CR with a single Pod

// 	gitops := &gitopsv1alpha1.GitOpsConfig{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       "GitOpsConfig",
// 			APIVersion: "eunomia.kohls.io/v1alpha1",
// 		},
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "gitops-events-periodic-success",
// 			Namespace: namespace,
// 		},
// 		Spec: gitopsv1alpha1.GitOpsConfigSpec{
// 			TemplateSource: gitopsv1alpha1.GitConfig{
// 				URI:        eunomiaURI,
// 				Ref:        eunomiaRef,
// 				ContextDir: "test/e2e/testdata/events/test-b",
// 			},
// 			ParameterSource: gitopsv1alpha1.GitConfig{
// 				URI:        eunomiaURI,
// 				Ref:        eunomiaRef,
// 				ContextDir: "test/e2e/testdata/empty-yaml",
// 			},
// 			Triggers: []gitopsv1alpha1.GitOpsTrigger{
// 				{
// 					Type: "Periodic",
// 					Cron: "*/1 * * * *",
// 				},
// 			},
// 			TemplateProcessorImage: "quay.io/kohlstechnology/eunomia-base:dev",
// 			ResourceHandlingMode:   "Apply",
// 			ResourceDeletionMode:   "Delete",
// 			ServiceAccountRef:      "eunomia-operator",
// 		},
// 	}
// 	gitops.Annotations = map[string]string{"gitopsconfig.eunomia.kohls.io/initialized": "true"}

// 	err = framework.Global.Client.Create(context.TODO(), gitops, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	err = WaitForPodWithImage(t, framework.Global, namespace, "hello-events-test-b", "hello-app:1.0", retryInterval, 2*time.Minute)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	// Step 3: Verify events

// 	// Verify there was an event emitted mentioning that Job finished successfully
// 	select {
// 	case event := <-events:
// 		if event.Reason != "JobSuccessful" ||
// 			event.DeprecatedSource.Component != "gitopsconfig-controller" {
// 			t.Errorf("got bad event: %v", event)
// 		}
// 	case <-time.After(10 * time.Second):
// 		t.Errorf("timeout waiting for JobSuccessful event")
// 	}
// }

func TestStatus_Failed(t *testing.T) {
	fmt.Println("START Failed")
	defer fmt.Println("END Failed")
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

	// Step 1: create a CR with an invalid URI

	gitops := &gitopsv1alpha1.GitOpsConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitOpsConfig",
			APIVersion: "eunomia.kohls.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitops-status-hello-failed",
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

	err = wait.Poll(retryInterval, 1*time.Minute, func() (done bool, err error) {
		fresh := gitopsv1alpha1.GitOpsConfig{}
		err = framework.Global.Client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: gitops.Name}, &fresh)
		if err != nil {
			return false, err
		}
		return fresh.Status.State != "", nil
	})
	if err != nil {
		t.Error(err)
	}

	// Step 3: watch Status till Failed & verify Status fields

	err = wait.Poll(retryInterval, 3*time.Minute, func() (done bool, err error) {
		fresh := gitopsv1alpha1.GitOpsConfig{}
		err = framework.Global.Client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: gitops.Name}, &fresh)
		if err != nil {
			return false, err
		}
		switch fresh.Status.State {
		case "InProgress":
			// TODO: check that fresh.Status.StartTime is in the past, but after whole test started
			if fresh.Status.CompletionTime != nil {
				t.Errorf("want CompletionTime==nil, got: %#v", fresh.Status)
			}
			return false, nil
		case "Failed":
			if fresh.Status.CompletionTime != nil {
				t.Errorf("want CompletionTime==nil, got: %#v", fresh.Status)
			}
			return true, nil
		default:
			t.Errorf("Unexpected State: %#v", fresh.Status)
			return false, nil
		}
	})
	if err != nil {
		t.Error(err)
	}
}
