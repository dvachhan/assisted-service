package lvm

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-version"
	"github.com/kelseyhightower/envconfig"
	"github.com/openshift/assisted-service/internal/common"
	"github.com/openshift/assisted-service/internal/oc"
	"github.com/openshift/assisted-service/internal/operators/api"
	"github.com/openshift/assisted-service/models"
	"github.com/openshift/assisted-service/pkg/conversions"
	logutil "github.com/openshift/assisted-service/pkg/log"
	"github.com/sirupsen/logrus"
)

// operator is an ODF LVM OLM operator plugin; it implements api.Operator
type operator struct {
	log       logrus.FieldLogger
	config    *Config
	extracter oc.Extracter
}

var Operator = models.MonitoredOperator{
	Name:             "lvm",
	OperatorType:     models.OperatorTypeOlm,
	Namespace:        "openshift-storage",
	SubscriptionName: "odf-lvm-operator",
	TimeoutSeconds:   30 * 60,
}

// NewLvmOperator creates new LvmOperator
func NewLvmOperator(log logrus.FieldLogger, extracter oc.Extracter) *operator {
	cfg := Config{}
	err := envconfig.Process(common.EnvConfigPrefix, &cfg)
	if err != nil {
		log.Fatal(err.Error())
	}
	return newLvmOperatorWithConfig(log, &cfg, extracter)
}

// newLvmOperatorWithConfig creates a new LvmOperator with given configuration
func newLvmOperatorWithConfig(log logrus.FieldLogger, config *Config, extracter oc.Extracter) *operator {
	return &operator{
		log:       log,
		config:    config,
		extracter: extracter,
	}
}

// GetName reports the name of an operator this Operator manages
func (o *operator) GetName() string {
	return Operator.Name
}

// GetDependencies provides a list of dependencies of the Operator
func (o *operator) GetDependencies(_ *common.Cluster) []string {
	return make([]string, 0)
}

// GetClusterValidationID returns cluster validation ID for the Operator
func (o *operator) GetClusterValidationID() string {
	return string(models.ClusterValidationIDLvmRequirementsSatisfied)
}

// GetHostValidationID returns host validation ID for the Operator
func (o *operator) GetHostValidationID() string {
	return string(models.HostValidationIDLvmRequirementsSatisfied)
}

// ValidateCluster always return "valid" result
func (o *operator) ValidateCluster(_ context.Context, cluster *common.Cluster) (api.ValidationResult, error) {
	if !common.IsSingleNodeCluster(cluster) {
		message := "ODF LVM operator is only supported for Single Node Openshift deployment"
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetClusterValidationID(), Reasons: []string{message}}, nil
	}

	var ocpVersion, minOpenshiftVersionForLvm *version.Version
	var err error

	ocpVersion, err = version.NewVersion(cluster.OpenshiftVersion)
	if err != nil {
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{err.Error()}}, nil
	}
	minOpenshiftVersionForLvm, err = version.NewVersion(LvmMinOpenshiftVersion)
	if err != nil {
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{err.Error()}}, nil
	}
	if ocpVersion.LessThan(minOpenshiftVersionForLvm) {
		message := "ODF LVM operator is only supported for openshift versions 4.12.0 and above"
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetClusterValidationID(), Reasons: []string{message}}, nil
	}

	return api.ValidationResult{Status: api.Success, ValidationId: o.GetClusterValidationID()}, nil
}

// ValidateHost always return "valid" result
func (o *operator) ValidateHost(ctx context.Context, cluster *common.Cluster, host *models.Host) (api.ValidationResult, error) {
	if host.Inventory == "" {
		message := "Missing Inventory in the host"
		return api.ValidationResult{Status: api.Pending, ValidationId: o.GetHostValidationID(), Reasons: []string{message}}, nil
	}

	inventory, err := common.UnmarshalInventory(host.Inventory)
	if err != nil {
		message := "Failed to get inventory from host"
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{message}}, err
	}

	// GetValidDiskCount counts the total number of valid disks in each host and return an error if we don't have the disk of required size
	diskCount, err := o.getValidDiskCount(inventory.Disks, host.InstallationDiskID)
	if err != nil {
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{err.Error()}}, nil
	}
	if diskCount == 0 {
		message := "Insufficient disks, ODF LVM requires at least one non-installation disk on the host"
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{message}}, nil
	}

	requirements, err := o.GetHostRequirements(ctx, cluster, host)
	if err != nil {
		message := fmt.Sprintf("Failed to get the host requirements for host with id %s", host.ID)
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{message, err.Error()}}, err
	}

	cpu := requirements.CPUCores
	if inventory.CPU.Count < cpu {
		message := fmt.Sprintf("Insufficient CPU to deploy ODF LVM. The required CPU count is %d but found %d", cpu, inventory.CPU.Count)
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{message}}, nil
	}

	memory := requirements.RAMMib
	memoryBytes := conversions.MibToBytes(memory)
	if inventory.Memory.UsableBytes < memoryBytes {
		usableMemory := conversions.BytesToMib(inventory.Memory.UsableBytes)
		message := fmt.Sprintf("Insufficient memory to deploy ODF LVM. The required memory is %d MiB but found %d MiB", memory, usableMemory)
		return api.ValidationResult{Status: api.Failure, ValidationId: o.GetHostValidationID(), Reasons: []string{message}}, nil
	}

	return api.ValidationResult{Status: api.Success, ValidationId: o.GetHostValidationID()}, nil
}

// GenerateManifests generates manifests for the operator
func (o *operator) GenerateManifests(_ *common.Cluster) (map[string][]byte, []byte, error) {
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
func (o *operator) GetHostRequirements(ctx context.Context, cluster *common.Cluster, _ *models.Host) (*models.ClusterHostRequirementsDetails, error) {
	log := logutil.FromContext(ctx, o.log)
	preflightRequirements, err := o.GetPreflightRequirements(ctx, cluster)
	if err != nil {
		log.WithError(err).Errorf("Cannot retrieve preflight requirements for cluster %s", cluster.ID)
		return nil, err
	}
	return preflightRequirements.Requirements.Master.Quantitative, nil
}

// GetPreflightRequirements returns operator hardware requirements that can be determined with cluster data only
func (o *operator) GetPreflightRequirements(_ context.Context, cluster *common.Cluster) (*models.OperatorHardwareRequirements, error) {
	return &models.OperatorHardwareRequirements{
		OperatorName: o.GetName(),
		Dependencies: o.GetDependencies(cluster),
		Requirements: &models.HostTypeHardwareRequirementsWrapper{
			Master: &models.HostTypeHardwareRequirements{
				Quantitative: &models.ClusterHostRequirementsDetails{
					CPUCores: o.config.LvmCPUPerHost,
					RAMMib:   o.config.LvmMemoryPerHostMiB,
				},
				Qualitative: []string{
					"At least 1 non-installation disk wih no partitions or filesystems",
				},
			},
			Worker: &models.HostTypeHardwareRequirements{
				Quantitative: &models.ClusterHostRequirementsDetails{},
			},
		},
	}, nil
}
