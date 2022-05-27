package odflvm

import (
	"bytes"
	"text/template"
)

const (
	Source string = "redhat-operators"
)

// Manifests returns manifests needed to deploy ODF LVM
func Manifests() (map[string][]byte, []byte, error) {
	odflvmSubs, err := subscription()

	if err != nil {
		return nil, nil, err
	}
	odflvmNs, err := namespace()
	if err != nil {
		return nil, nil, err
	}
	odflvmGrp, err := group()
	if err != nil {
		return nil, nil, err
	}
	odflvmLVMC, err := lvmcluster()
	if err != nil {
		return nil, nil, err
	}

	openshiftManifests := make(map[string][]byte)

	openshiftManifests["50_openshift-odflvm_subscription.yaml"] = odflvmSubs
	openshiftManifests["50_openshift-odflvm_ns.yaml"] = odflvmNs
	openshiftManifests["50_openshift-odflvm_operator_group.yaml"] = odflvmGrp
	return openshiftManifests, []byte(odflvmLVMC), nil
}

func subscription() ([]byte, error) {
	data := map[string]string{
		"OPERATOR_NAMESPACE":         Operator.Namespace,
		"OPERATOR_SUBSCRIPTION_NAME": Operator.SubscriptionName,
		"OPERATOR_SOURCE":            Source,
	}
	return executeTemplate(data, "odflvmSubscription", odflvmSubscription)
}

func namespace() ([]byte, error) {
	data := map[string]string{
		"OPERATOR_NAMESPACE": Operator.Namespace,
	}
	return executeTemplate(data, "odflvmNamespace", odflvmNamespace)
}

func group() ([]byte, error) {
	data := map[string]string{
		"OPERATOR_NAMESPACE": Operator.Namespace,
	}
	return executeTemplate(data, "odflvmGroup", odflvmOperatorGroup)
}

func lvmcluster() ([]byte, error) {
	data := map[string]string{
		"OPERATOR_NAMESPACE": Operator.Namespace,
	}
	return executeTemplate(data, "odflvmLVMCluster", odflvmLVMCluster)
}

func executeTemplate(data map[string]string, contentName, content string) ([]byte, error) {
	tmpl, err := template.New(contentName).Parse(content)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const odflvmSubscription = `operators.coreos.com/v1alpha1
kind: Subscription
metadata:
name: "{{.OPERATOR_SUBSCRIPTION_NAME}}"
namespace: "{{.OPERATOR_NAMESPACE}}"
spec:
  installPlanApproval: Automatic
  name: odf-lvm-operator
  source: "{{.OPERATOR_SOURCE}}"
  sourceNamespace: openshift-marketplace
  startingCSV: odf-lvm-operator.v4.10.0`

const odflvmNamespace = `apiVersion: v1
kind: Namespace
metadata:
name: "{{.OPERATOR_NAMESPACE}}"
labels:
	openshift.io/cluster-monitoring: "true"`

const odflvmOperatorGroup = `operators.coreos.com/v1
kind: OperatorGroup
metadata:
	name: openshift-storage-operatorgroup
	namespace: "{{.OPERATOR_NAMESPACE}}"
spec:
	targetNamespaces:
	- "{{.OPERATOR_NAMESPACE}}"`

const odflvmLVMCluster = `apiVersion: lvm.topolvm.io/v1alpha1
kind: LVMCluster
metadata:
	name: lvmcluster-sample
	namespace: "{{.OPERATOR_NAMESPACE}}"
spec:
	storage:
	deviceClasses:
	- name: vg1`
