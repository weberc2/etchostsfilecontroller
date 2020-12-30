package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const interval = 30 * time.Second

func syncHostsFile(filePath string, ingresses []*v1.Ingress) error {
	const annotation = "etchostsfilecontroller.weberc2.com/dns-name"
	entries := make([]string, 0, len(ingresses))

OUTER:
	for _, ingress := range ingresses {
		log.Printf("INFO Found ingress %s.%s", ingress.Namespace, ingress.Name)

		dnsName := ingress.Annotations[annotation]
		if dnsName == "" {
			log.Printf(
				"INFO missing annotation %s on ingress %s.%s; skipping",
				annotation,
				ingress.Namespace,
				ingress.Name,
			)
			continue
		}

		for _, i := range ingress.Status.LoadBalancer.Ingress {
			if i.IP != "" {
				entries = append(
					entries,
					fmt.Sprintf("%s\t%s", i.IP, dnsName),
				)
				continue OUTER
			}
		}

		log.Printf(
			"WARNING couldn't find an address for ingress '%s.%s'",
			ingress.Namespace,
			ingress.Name,
		)
	}

	if err := ioutil.WriteFile(
		filePath,
		[]byte(strings.Join(entries, "\n")),
		0644,
	); err != nil {
		return err
	}

	return nil
}

func main() {
	filePath := os.Getenv("HOSTS_FILE")
	if filePath == "" {
		log.Panic("$HOSTS_FILE not set or empty")
	}

	// If KUBECONFIG is unset or empty, `BuildConfigFromFlags` won't try to
	// open the file; this is the correct case for running the program inside
	// of a pod.
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		log.Panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panic(err.Error())
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Networking().V1().Ingresses().Informer()
	lister := factory.Networking().V1().Ingresses().Lister()

	// We have to run the informer in order for the lister to have entries.
	// The informer seems to be the thing that populates the cache from which
	// the lister reads.
	stop := make(chan struct{})
	defer close(stop)
	log.Printf("Starting informer")
	go informer.Run(stop)
	time.Sleep(1 * time.Second) // wait a second for the cache to populate

	log.Printf("Starting lister loop")
	for {
		ingresses, err := lister.List(labels.Everything())
		if err != nil {
			log.Printf("ERROR listing ingresses: %v", err)
			time.Sleep(interval)
			continue
		}

		log.Printf("INFO found %d ingresses", len(ingresses))

		if err := syncHostsFile(filePath, ingresses); err != nil {
			log.Println("ERROR", err)
		}
		time.Sleep(interval)
	}
}
