package odflvm

import (
	"context"
	"fmt"

	"github.com/openshift/assisted-service/internal/common"
	"github.com/openshift/assisted-service/internal/operators/api"
	"github.com/openshift/assisted-service/models"
	"github.com/openshift/assisted-service/pkg/conversions"
	"github.com/sirupsen/logrus"
)

// operator is an ODF LVM OLM operator plugin; it implements api.Operator
type operator struct {
	log    logrus.FieldLogger
	config Config
}

var Operator = models.MonitoredOperator{
	Name:             "odflvm",
	OperatorType:     models.OperatorTypeOlm,
	Namespace:        "openshift-storage",
	SubscriptionName: "odf-lvm-operator",
	TimeoutSeconds:   70 * 60,
}

// New ODFLVMOperator creates new instance of an ODF LVM Operator installation plugin
func NewODFLVMOperator() *operator {
	return &operator{}
}

// GetName reports the name of an operator this Operator manages
func (o *operator) GetName() string {
	return Operator.Name
}

// GetDependencies provides a list of dependencies of the Operator
func (o *operator) GetDependencies() []string {
	return make([]string, 0)
}

// GetClusterValidationID returns cluster validation ID for the Operator
func (o *operator) GetClusterValidationID() string {
	return string(models.ClusterValidationIDLsoRequirementsSatisfied)
}

// GetHostValidationID returns host validation ID for the Operator
func (o *operator) GetHostValidationID() string {
	return string(models.HostValidationIDLsoRequirementsSatisfied)
}

// ValidateCluster always return "valid" result
func (o *operator) ValidateCluster(_ context.Context, cluster *common.Cluster) (api.ValidationResult, error) {
	if common.IsSingleNodeCluster(cluster) {
		return api.ValidationResult{Status: api.Success, ValidationId: o.GetClusterValidationID(), Reasons: []string{}}, nil
	} else {
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetClusterValidationID(), Reasons: []string{}}, nil
	}
}

// ValidateHost always return "valid" result
func (o *operator) ValidateHost(ctx context.Context, cluster *common.Cluster, host *models.Host) (api.ValidationResult, error) {
	if host.Inventory == "" {
		return api.ValidationResult{Status: api.Pending, ValidationId: o.GetHostValidationID(), Reasons: []string{"Missing Inventory in the host"}}, nil
	}
	inventory, err := common.UnmarshalInventory(host.Inventory)
	if err != nil {
		message := "Failed to get inventory from host"
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{message}}, err
	}
	// GetValidDiskCount counts the total number of valid disks in each host
	diskCount := o.getValidDiskCount(inventory.Disks, host.InstallationDiskID)
	if diskCount == 0 {
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{"Insufficient disks, ODF LVM requires at least one non-bootable disk on the host"}}, nil
	}

	requirements, err := o.GetHostRequirements(ctx, cluster, host)
	if err != nil {
		message := fmt.Sprintf("Failed to get host requirements for host with id %s", host.ID)
		o.log.Error(message)
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{message, err.Error()}}, err
	}

	cpu := requirements.CPUCores
	if inventory.CPU.Count < cpu {
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{fmt.Sprintf("Insufficient CPU to deploy ODF LVM. Required CPU count is %d but found %d ", cpu, inventory.CPU.Count)}}, nil
	}

	mem := requirements.RAMMib
	memBytes := conversions.MibToBytes(mem)
	if inventory.Memory.UsableBytes < memBytes {
		usableMemory := conversions.BytesToMib(inventory.Memory.UsableBytes)
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{fmt.Sprintf("Insufficient memory to deploy ODF LVM. Required memory is %d MiB but found %d MiB", mem, usableMemory)}}, nil
	}

	return api.ValidationResult{Status: api.Success, ValidationId: o.GetHostValidationID()}, nil
}

// GenerateManifests generates manifests for the operator
func (o *operator) GenerateManifests() (map[string][]byte, []byte, error) {
	return Manifests()
}

// GetProperties provides description of operator properties: none required
func (o *operator) GetProperties() models.OperatorProperties {
	return models.OperatorProperties{}
}

// GetMonitoredOperator returns MonitoredOperator corresponding to the LSO
func (o *operator) GetMonitoredOperator() *models.MonitoredOperator {
	return &Operator
}

// GetHostRequirements provides operator's requirements towards the host
func (o *operator) GetHostRequirements(ctx context.Context, cluster *common.Cluster, host *models.Host) (*models.ClusterHostRequirementsDetails, error) {
	return &models.ClusterHostRequirementsDetails{
		CPUCores: o.config.ODFLVMCPUPerHost,
		RAMMib:   o.config.ODFLVMMemoryMiBPerHost,
	}, nil
}

// GetPreflightRequirements returns operator hardware requirements that can be determined with cluster data only
func (o *operator) GetPreflightRequirements(context.Context, *common.Cluster) (*models.OperatorHardwareRequirements, error) {
	return &models.OperatorHardwareRequirements{
		OperatorName: o.GetName(),
		Dependencies: o.GetDependencies(),
		Requirements: &models.HostTypeHardwareRequirementsWrapper{
			Master: &models.HostTypeHardwareRequirements{
				Quantitative: &models.ClusterHostRequirementsDetails{
					CPUCores: o.config.ODFLVMCPUPerHost,
					RAMMib:   o.config.ODFLVMMemoryMiBPerHost,
				},
				Qualitative: []string{
					"At least 1 non-bootable disk wih no partitions or filesystems",
				},
			},
		},
	}, nil
}
