package lvm

import (
	"context"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/openshift/assisted-service/internal/common"
	"github.com/openshift/assisted-service/internal/operators/api"
	"github.com/openshift/assisted-service/models"
	"github.com/openshift/assisted-service/pkg/conversions"
)

var _ = Describe("Lvm Operator", func() {
	var (
		ctx                       = context.TODO()
		operator                  = NewLvmOperator(common.GetTestLog(), nil)
		diskID1                   = "/dev/disk/by-id/test-disk-1"
		diskID2                   = "/dev/disk/by-id/test-disk-2"
		hostWithNoInventory       = &models.Host{}
		hostWithInsufficientDisks = &models.Host{
			InstallationDiskID: diskID1,
			Inventory: Inventory(&InventoryResources{
				Cpus: 12,
				Ram:  32 * conversions.GiB,
				Disks: []*models.Disk{
					{
						SizeBytes: 20 * conversions.GB,
						DriveType: models.DriveTypeHDD,
						ID:        diskID1,
					},
				},
			}),
		}
		hostWithInsufficientCPU = &models.Host{
			InstallationDiskID: diskID1,
			Inventory: Inventory(&InventoryResources{
				Cpus: 0,
				Ram:  32 * conversions.GiB,
				Disks: []*models.Disk{
					{
						SizeBytes: 20 * conversions.GB,
						DriveType: models.DriveTypeHDD,
						ID:        diskID1,
					},
					{
						SizeBytes: 40 * conversions.GB,
						DriveType: models.DriveTypeSSD,
						ID:        diskID2,
					},
				},
			}),
		}
		hostWithInsufficientRAM = &models.Host{
			InstallationDiskID: diskID1,
			Inventory: Inventory(&InventoryResources{
				Cpus: 12,
				Ram:  1199 * conversions.MiB,
				Disks: []*models.Disk{
					{
						SizeBytes: 20 * conversions.GB,
						DriveType: models.DriveTypeHDD,
						ID:        diskID1,
					},
					{
						SizeBytes: 40 * conversions.GB,
						DriveType: models.DriveTypeSSD,
						ID:        diskID2,
					},
				},
			}),
		}
		hostWithSufficientResources = &models.Host{
			InstallationDiskID: diskID1,
			Inventory: Inventory(&InventoryResources{
				Cpus: 12,
				Ram:  32 * conversions.GiB,
				Disks: []*models.Disk{
					{
						SizeBytes: 20 * conversions.GB,
						DriveType: models.DriveTypeHDD,
						ID:        diskID1,
					},
					{
						SizeBytes: 40 * conversions.GB,
						DriveType: models.DriveTypeSSD,
						ID:        diskID2,
					},
				},
			}),
		}
	)

	Context("GetHostRequirements", func() {
		table.DescribeTable("get host requirements when ", func(cluster *common.Cluster, host *models.Host, expectedResult *models.ClusterHostRequirementsDetails) {
			res, _ := operator.GetHostRequirements(ctx, cluster, host)
			Expect(res).Should(Equal(expectedResult))
		},
			table.Entry("host",
				&common.Cluster{Cluster: models.Cluster{Hosts: []*models.Host{hostWithSufficientResources}}},
				hostWithSufficientResources,
				&models.ClusterHostRequirementsDetails{CPUCores: operator.config.LvmCPUPerHost, RAMMib: operator.config.LvmMemoryPerHostMiB},
			),
		)
	})

	Context("ValidateHost", func() {
		table.DescribeTable("validate host when ", func(cluster *common.Cluster, host *models.Host, expectedResult api.ValidationResult) {
			res, _ := operator.ValidateHost(ctx, cluster, host)
			Expect(res).Should(Equal(expectedResult))
		},
			table.Entry("host with no inventory",
				&common.Cluster{Cluster: models.Cluster{Hosts: []*models.Host{hostWithNoInventory}}},
				hostWithNoInventory,
				api.ValidationResult{Status: api.Pending, ValidationId: operator.GetHostValidationID(), Reasons: []string{"Missing Inventory in the host"}},
			),
			table.Entry("host with insufficient disks",
				&common.Cluster{Cluster: models.Cluster{Hosts: []*models.Host{hostWithInsufficientDisks}}},
				hostWithInsufficientDisks,
				api.ValidationResult{Status: api.Failure, ValidationId: operator.GetHostValidationID(), Reasons: []string{"Insufficient disks, ODF LVM requires at least one non-installation disk on the host"}},
			),
			table.Entry("host with insufficient CPU",
				&common.Cluster{Cluster: models.Cluster{Hosts: []*models.Host{hostWithInsufficientCPU}}},
				hostWithInsufficientCPU,
				api.ValidationResult{Status: api.Failure, ValidationId: operator.GetHostValidationID(), Reasons: []string{"Insufficient CPU to deploy ODF LVM. The required CPU count is 1 but found 0"}},
			),
			table.Entry("master with insufficient RAM",
				&common.Cluster{Cluster: models.Cluster{Hosts: []*models.Host{hostWithInsufficientRAM}}},
				hostWithInsufficientRAM,
				api.ValidationResult{Status: api.Failure, ValidationId: operator.GetHostValidationID(), Reasons: []string{"Insufficient memory to deploy ODF LVM. The required memory is 1200 MiB but found 1199 MiB"}},
			),
			table.Entry("master with sufficient resources",
				&common.Cluster{Cluster: models.Cluster{Hosts: []*models.Host{hostWithSufficientResources}}},
				hostWithSufficientResources,
				api.ValidationResult{Status: api.Success, ValidationId: operator.GetHostValidationID()},
			),
		)
	})
	Context("ValidateCluster", func() {
		fullHaMode := models.ClusterHighAvailabilityModeFull
		noneHaMode := models.ClusterHighAvailabilityModeNone

		table.DescribeTable("validate cluster when ", func(cluster *common.Cluster, expectedResult api.ValidationResult) {
			res, _ := operator.ValidateCluster(ctx, cluster)
			Expect(res).Should(Equal(expectedResult))
		},
			table.Entry("High Availability Mode Full",
				&common.Cluster{Cluster: models.Cluster{HighAvailabilityMode: &fullHaMode, Hosts: []*models.Host{hostWithSufficientResources, hostWithSufficientResources}, OpenshiftVersion: "4.12.0"}},
				api.ValidationResult{Status: api.Failure, ValidationId: operator.GetHostValidationID(), Reasons: []string{"ODF LVM operator is only supported for Single Node Openshift deployment"}},
			),
			table.Entry("High Availability Mode None and Openshift version less than minimal",
				&common.Cluster{Cluster: models.Cluster{HighAvailabilityMode: &noneHaMode, Hosts: []*models.Host{hostWithSufficientResources}, OpenshiftVersion: "4.10.0"}},
				api.ValidationResult{Status: api.Failure, ValidationId: operator.GetHostValidationID(), Reasons: []string{"ODF LVM operator is only supported for openshift versions 4.12.0 and above"}},
			),
			table.Entry("High Availability Mode None and Openshift version more than minimal",
				&common.Cluster{Cluster: models.Cluster{HighAvailabilityMode: &noneHaMode, Hosts: []*models.Host{hostWithSufficientResources}, OpenshiftVersion: "4.12.0"}},
				api.ValidationResult{Status: api.Success, ValidationId: operator.GetHostValidationID()},
			),
		)
	})
})
