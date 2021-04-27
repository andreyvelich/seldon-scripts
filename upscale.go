package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	seldonv1 "github.com/seldonio/seldon-core/operator/apis/machinelearning.seldon.io/v1"
)

var (
	timeout  = 30 * time.Minute
	replicas = int32(2)
)

func main() {
	deployYAMLPath := flag.String("f", "", "Path to the Seldon Deployment YAML file")
	flag.Parse()

	// Verify that file path is set.
	if *deployYAMLPath == "" {
		log.Fatal("Path for Seldon Deployment YAML must be set")
	}

	log.Printf("Seldon model Deployment file path: %v", *deployYAMLPath)

	// Get the new controller-runtime and clientset client.
	client, clientset, err := getClient()
	if err != nil {
		log.Fatalf("Unable to get Kubernetes client, error: %v", err)
	}

	// Step 1. Create the Seldon Deployment.
	name, UID, namespace, err := createSeldonDeployment(client, *deployYAMLPath)
	if err != nil {
		log.Fatalf("Unable to create Seldon Deployment, error: %v", err)
	}
	log.Printf("Seldon Deployment has been created. Name: %v, namespace: %v", name, namespace)

	// Step 2. Print the Kubernetes events for the Seldon Deployment.
	go printEvents(clientset, UID, namespace)

	// Step 3. Wait until the Seldon Deployment is available.
	err = waitSeldonDeploymentAvailable(client, name, namespace)
	if err != nil {
		log.Fatalf("Unable to wait for available Seldon Deployment, error: %v", err)
	}

	// Step 4. Scale replicas to 2 for the Seldon Deployment.
	err = scaleSeldonDeployment(client, name, namespace, &replicas)
	if err != nil {
		log.Fatalf("Unable to scale Seldon Deployment, error: %v", err)
	}
	log.Printf("Seldon Deployment is scaling to %v replicas", replicas)

	// Step 5. Wait until the Seldon Deployment is available.
	err = waitSeldonDeploymentAvailable(client, name, namespace)
	if err != nil {
		log.Fatalf("Unable to wait for available Seldon Deployment, error: %v", err)
	}
	log.Printf("Seldon Deployment scaled with %v replicas", replicas)

	// Step 6. Delete the Seldon Deployment.
	err = deleteSeldonDeployment(client, name, namespace)
	if err != nil {
		log.Fatalf("Unable to delete Seldon Deployment, error: %v", err)
	}
	log.Print("Seldon Deployment has been deleted")
}

// Get the controller-runtime and client-go client.
// Specify -kubeconfig flag to set the custom config path.
func getClient() (client.Client, *kubernetes.Clientset, error) {

	// Add Seldon types to scheme.
	seldonv1.AddToScheme(scheme.Scheme)

	config, err := config.GetConfig()
	if err != nil {
		log.Print("Unable to get kubeconfig")
		return nil, nil, err
	}

	// Create the new controller-runtime client.
	client, err := client.New(config, client.Options{})
	if err != nil {
		log.Print("Unable to create new client")
		return nil, nil, err
	}

	// Create the new ClientSet for events.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Print("Unable to create new clientset")
		return nil, nil, err
	}

	return client, clientset, nil
}

// Print events for the Seldon Deployment.
func printEvents(clientset *kubernetes.Clientset, UID, namespace string) {

	// To not print previous events for the same object name, we will take object's UID.
	watchList := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"events",
		namespace,
		fields.OneTermEqualSelector("involvedObject.uid", UID),
	)
	_, ctrl := cache.NewInformer(
		watchList,
		&v1.Event{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				event, _ := obj.(*v1.Event)
				log.Printf("Event. Type: %v, Reason: %v, Message: %v\n", event.Type, event.Reason, event.Message)
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go ctrl.Run(stop)
	for {
		time.Sleep(time.Second)
	}
}

// Create Seldon Deployment from the given YAML path.
// Returns Deployment name, UID and namespace
func createSeldonDeployment(client client.Client, deployYAMLPath string) (string, string, string, error) {

	// Read file to byte array.
	byteFile, err := os.Open(deployYAMLPath)
	if err != nil {
		log.Printf("Unable to read file: %v", deployYAMLPath)
		return "", "", "", err
	}

	// Convert byte array to SeldonDeployment object
	sd := &seldonv1.SeldonDeployment{}
	if err := k8syaml.NewYAMLOrJSONDecoder(byteFile, 1024).Decode(sd); err != nil {
		log.Print("Unable to convert YAML to SeldonDeployment")
		return "", "", "", err
	}
	// Set the default namespace.
	if sd.Namespace == "" {
		sd.Namespace = "default"
	}

	// Create SeldonDeployment.
	if err := client.Create(context.Background(), sd); err != nil {
		log.Printf("Unable to create Seldon Deployment %v", sd)
		return "", "", "", err
	}

	return sd.Name, string(sd.UID), sd.Namespace, nil
}

// Wait until Seldon Deployment is available.
func waitSeldonDeploymentAvailable(client client.Client, name, namespace string) error {
	for endTime := time.Now().Add(timeout); time.Now().Before(endTime); {
		sd := &seldonv1.SeldonDeployment{}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, sd); err != nil {
			log.Print("Unable to Get Seldon Deployment")
			return err
		}
		if sd.Status.State == seldonv1.StatusStateAvailable {
			log.Print("Seldon Deployment is available")
			return nil
		}
		// Print only if State is not empty.
		if sd.Status.State != "" {
			log.Printf("Seldon Deployment is not available, current status: %v. Sleep for 5 seconds", sd.Status.State)
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout to get available status for Seldon Deployment")
}

// Scale Seldon Deployment to the replicasCount replicas.
func scaleSeldonDeployment(client client.Client, name, namespace string, replicasCount *int32) error {
	sd := &seldonv1.SeldonDeployment{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, sd); err != nil {
		log.Print("Unable to Get Seldon Deployment")
		return err
	}
	// Modify Replicas and update resource.
	sd.Spec.Replicas = replicasCount
	if err := client.Update(context.TODO(), sd); err != nil {
		log.Print("Unable to Update Seldon Deployment")
		return err
	}
	// Wait until Seldon Deployment status is changed to Creating.
	// Controller takes some time to reconcile this change.
	for endTime := time.Now().Add(timeout); time.Now().Before(endTime); {
		sd := &seldonv1.SeldonDeployment{}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, sd); err != nil {
			log.Print("Unable to Get Seldon Deployment")
			return err
		}
		if sd.Status.State == seldonv1.StatusStateCreating {
			return nil
		}
	}
	return fmt.Errorf("timeout to get creating status for Seldon Deployment")
}

// Delete the Seldon Deployment.
func deleteSeldonDeployment(client client.Client, name, namespace string) error {
	sd := &seldonv1.SeldonDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := client.Delete(context.TODO(), sd); err != nil {
		log.Print("Unable to Delete Seldon Deployment")
		return err
	}
	return nil
}
