/*
Copyright 2026.

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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tazhate/chainplane/test/utils"
)

// namespace where the project is deployed in
const namespace = "chainplane-system"

// serviceAccountName created for the project
const serviceAccountName = "chainplane-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "chainplane-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "chainplane-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace with privileged security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=privileged")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching previous controller manager pod logs (crash logs)")
			prevCmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace, "--previous")
			prevLogs, prevErr := utils.Run(prevCmd)
			if prevErr == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Previous controller logs (crash):\n %s", prevLogs)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=chainplane-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
				"--dry-run=client", "-o", "yaml",
			)
			yamlOut, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to render ClusterRoleBinding")
			applyCmd := exec.Command("kubectl", "apply", "-f", "-")
			applyCmd.Stdin = strings.NewReader(yamlOut)
			_, err = utils.Run(applyCmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("validating that the ServiceMonitor for Prometheus is applied in the namespace")
			if !skipPrometheusInstall {
				cmd = exec.Command("kubectl", "get", "ServiceMonitor", "-n", namespace)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "ServiceMonitor should exist")
			}

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})

	Context("BlockchainNode CR lifecycle", func() {
		const testNS = namespace

		// waitForControllerReady waits until the controller-manager pod is Running.
		// This handles the case where the pod restarts (e.g., leader election loss)
		// and ensures the controller is ready before applying CRs.
		waitForControllerReady := func() {
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}{{ \"\\n\" }}"+
						"{{ end }}{{ end }}",
					"-n", namespace,
				)
				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1))
				controllerPodName = podNames[0]

				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"))
			}).Should(Succeed())
		}

		BeforeEach(func() {
			waitForControllerReady()
		})

		// cleanupCR deletes a BlockchainNode CR by name and waits for it to disappear.
		cleanupCR := func(name string) {
			cmd := exec.Command("kubectl", "delete", "blockchainnode", name,
				"-n", testNS, "--ignore-not-found=true", "--timeout=60s")
			_, _ = utils.Run(cmd)
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "blockchainnode", name,
					"-n", testNS, "-o", "name")
				output, err := utils.Run(cmd)
				// Either error (not found) or empty output means it's gone
				if err != nil {
					return
				}
				g.Expect(output).To(BeEmpty())
			}, 2*time.Minute, time.Second).Should(Succeed())
		}

		// applyCR applies a BlockchainNode CR from a YAML string and returns the CR name.
		applyCR := func(yaml string) string {
			tmpFile := filepath.Join("/tmp", fmt.Sprintf("e2e-cr-%d.yaml", time.Now().UnixNano()))
			err := os.WriteFile(tmpFile, []byte(yaml), 0o644)
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile)

			cmd := exec.Command("kubectl", "apply", "-f", tmpFile, "-n", testNS)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply BlockchainNode CR")
			return tmpFile
		}

		// waitForStatefulSet polls until a StatefulSet with the given name exists.
		waitForStatefulSet := func(name string) {
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "statefulset", name,
					"-n", testNS, "-o", "name")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "StatefulSet %s not found yet", name)
				g.Expect(output).To(ContainSubstring(name))
			}, 2*time.Minute, time.Second).Should(Succeed())
		}

		// --- Scenario 1: Create and verify resources ---
		It("should create StatefulSet, Service, and ConfigMap for a bitcoin CR", func() {
			crName := "e2e-btc-resources"
			cr := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: bitcoin
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
    limits:
      cpu: "200m"
      memory: 256Mi
  rpc:
    enabled: true
    port: 8332
`, crName, testNS)

			By("applying the bitcoin BlockchainNode CR")
			applyCR(cr)
			defer cleanupCR(crName)

			By("verifying that a StatefulSet is created")
			waitForStatefulSet(crName)

			By("verifying that a ClusterIP Service is created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", crName,
					"-n", testNS, "-o", "jsonpath={.spec.type}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Service not found")
				g.Expect(output).To(Equal("ClusterIP"))
			}, 2*time.Minute, time.Second).Should(Succeed())

			By("verifying that a ConfigMap is created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", crName+"-config",
					"-n", testNS, "-o", "name")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "ConfigMap not found")
				g.Expect(output).To(ContainSubstring(crName))
			}, 2*time.Minute, time.Second).Should(Succeed())
		})

		// --- Scenario 2: Phase lifecycle ---
		It("should set initial phase to Pending or Syncing", func() {
			crName := "e2e-btc-phase"
			cr := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: bitcoin
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
  rpc:
    enabled: true
    port: 8332
`, crName, testNS)

			By("applying the bitcoin BlockchainNode CR")
			applyCR(cr)
			defer cleanupCR(crName)

			By("verifying initial phase is Pending or Syncing")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "blockchainnode", crName,
					"-n", testNS, "-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(SatisfyAny(
					Equal("Pending"),
					Equal("Syncing"),
				), "expected phase Pending or Syncing, got: %s", output)
			}, 2*time.Minute, time.Second).Should(Succeed())
		})

		// --- Scenario 3: Scale to zero ---
		It("should scale StatefulSet to zero replicas when replicas=0", func() {
			crName := "e2e-btc-scale-zero"
			cr := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: bitcoin
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  replicas: 1
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
  rpc:
    enabled: true
    port: 8332
`, crName, testNS)

			By("applying the bitcoin BlockchainNode CR with replicas=1")
			applyCR(cr)
			defer cleanupCR(crName)

			By("waiting for the StatefulSet to be created")
			waitForStatefulSet(crName)

			By("patching the CR to set replicas=0")
			cmd := exec.Command("kubectl", "patch", "blockchainnode", crName,
				"-n", testNS, "--type=merge", "-p", `{"spec":{"replicas":0}}`)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch replicas to 0")

			By("verifying StatefulSet replicas is 0")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "statefulset", crName,
					"-n", testNS, "-o", "jsonpath={.spec.replicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("0"), "StatefulSet replicas should be 0")
			}, 2*time.Minute, time.Second).Should(Succeed())
		})

		// --- Scenario 4: Delete cleanup ---
		It("should remove all child resources when CR is deleted", func() {
			crName := "e2e-btc-delete"
			cr := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: bitcoin
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
  rpc:
    enabled: true
    port: 8332
`, crName, testNS)

			By("applying the bitcoin BlockchainNode CR")
			applyCR(cr)

			By("waiting for the StatefulSet to be created")
			waitForStatefulSet(crName)

			By("deleting the BlockchainNode CR")
			cmd := exec.Command("kubectl", "delete", "blockchainnode", crName,
				"-n", testNS, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete BlockchainNode CR")

			By("verifying StatefulSet is removed")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "statefulset", crName,
					"-n", testNS)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "StatefulSet should be deleted")
			}, 2*time.Minute, time.Second).Should(Succeed())

			By("verifying Service is removed")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", crName,
					"-n", testNS)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "Service should be deleted")
			}, 2*time.Minute, time.Second).Should(Succeed())

			By("verifying ConfigMap is removed")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", crName+"-config",
					"-n", testNS)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "ConfigMap should be deleted")
			}, 2*time.Minute, time.Second).Should(Succeed())
		})

		// --- Scenario 5: Multiple chains ---
		It("should manage multiple blockchain CRs simultaneously", func() {
			btcName := "e2e-multi-btc"
			ethName := "e2e-multi-eth"

			btcCR := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: bitcoin
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
  rpc:
    enabled: true
    port: 8332
`, btcName, testNS)

			ethCR := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: ethereum
  network: mainnet
  nodeType: rpc
  nodeGroup: heavy
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
  rpc:
    enabled: true
    port: 8545
`, ethName, testNS)

			By("applying both bitcoin and ethereum CRs")
			applyCR(btcCR)
			defer cleanupCR(btcName)
			applyCR(ethCR)
			defer cleanupCR(ethName)

			By("verifying bitcoin StatefulSet exists")
			waitForStatefulSet(btcName)

			By("verifying ethereum StatefulSet exists")
			waitForStatefulSet(ethName)

			By("verifying both CRs have distinct chain fields")
			cmd := exec.Command("kubectl", "get", "blockchainnode", btcName,
				"-n", testNS, "-o", "jsonpath={.spec.chain}")
			btcChain, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(btcChain).To(Equal("bitcoin"))

			cmd = exec.Command("kubectl", "get", "blockchainnode", ethName,
				"-n", testNS, "-o", "jsonpath={.spec.chain}")
			ethChain, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(ethChain).To(Equal("ethereum"))
		})

		// --- Scenario 6: Custom image override ---
		It("should use a custom image when specified in the CR", func() {
			crName := "e2e-btc-custom-image"
			customRepo := "my-registry/custom-bitcoind"
			customTag := "v99.0.0-test"
			cr := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: bitcoin
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  image:
    repository: %s
    tag: %s
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
  rpc:
    enabled: true
    port: 8332
`, crName, testNS, customRepo, customTag)

			By("applying the bitcoin CR with a custom image")
			applyCR(cr)
			defer cleanupCR(crName)

			By("waiting for the StatefulSet to be created")
			waitForStatefulSet(crName)

			By("verifying the StatefulSet uses the custom image")
			expectedImage := fmt.Sprintf("%s:%s", customRepo, customTag)
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "statefulset", crName,
					"-n", testNS,
					"-o", "jsonpath={.spec.template.spec.containers[0].image}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(expectedImage),
					"StatefulSet container image should be %s, got %s", expectedImage, output)
			}, 2*time.Minute, time.Second).Should(Succeed())
		})

		// --- Scenario 7: Invalid CR rejection ---
		It("should reject a CR with an unsupported chain value", func() {
			crName := "e2e-invalid-chain"
			cr := fmt.Sprintf(`
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: %s
  namespace: %s
spec:
  chain: notachain
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  storage:
    size: 10Gi
  resources:
    requests:
      cpu: "100m"
      memory: 128Mi
  rpc:
    enabled: true
    port: 8332
`, crName, testNS)

			By("applying a CR with unsupported chain 'notachain'")
			tmpFile := filepath.Join("/tmp", fmt.Sprintf("e2e-invalid-%d.yaml", time.Now().UnixNano()))
			err := os.WriteFile(tmpFile, []byte(cr), 0o644)
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile)

			cmd := exec.Command("kubectl", "apply", "-f", tmpFile, "-n", testNS)
			output, err := utils.Run(cmd)

			// The CR should be rejected by CRD validation (enum constraint on chain field).
			// If webhooks are not running, the CRD OpenAPI validation still rejects invalid enums.
			Expect(err).To(HaveOccurred(),
				"Expected CR with unsupported chain to be rejected, but it was accepted. Output: %s", output)
			_, _ = fmt.Fprintf(GinkgoWriter, "Invalid CR correctly rejected: %s\n", output)
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
