package vms

const (
	BasicVmID                                    = "123"
	TwoDisksVmID                                 = "two-disks"
	InvalidDiskID                                = "invalid"
	InvalidNicInterfaceVmIDPrefix                = "nic-interface-"
	UnsupportedStatusVmIDPrefix                  = "unsupported-status-"
	UnsupportedArchitectureVmID                  = "unsupported-s390x-architecture"
	NicPassthroughVmID                           = "nic-passthrough"
	IlleagalImagesVmID                           = "illegal-images"
	KubevirtOriginVmID                           = "kubevirt-origin"
	MigratablePlacementPolicyAffinityVmID        = "migratable-placement-policy-affinity"
	UsbEnabledVmID                               = "usb-enabled"
	UnsupportedDiag288WatchdogVmID               = "unsupported-diag288-watchdog"
	BasicNetworkVmID                             = "basic-network"
	TwoNetworksVmID                              = "two-networks"
	UnsupportedDiskAttachmentInterfaceVmIDPrefix = "unsupported-disk-attachment-interface-"
	UnsupportedDiskInterfaceVmIDPrefix           = "unsupported-disk-interface-"
	ScsiReservationDiskAttachmentVmID            = "scsi-reservation-disk-attachment"
	ScsiReservationDiskVmID                      = "scsi-reservation-disk"
	LUNStorageDiskVmID                           = "lun-storage-disk"
	IllegalDiskStatusVmIDPrefix                  = "illegal-disk-status-"
	UnsupportedDiskStorageTypeVmIDPrefix         = "unsupported-disk-storage-type-"
	UnsupportedDiskSGIOVmIDPrefix                = "unsupported-disk-sgio-type-"
	UnsupportedTimezoneVmID                      = "unsupported-timezone"
	UtcCompatibleTimeZoneVmID                    = "timezone-vm"
	BIOSTypeVmIDPrefix                           = "bios-type-"
	ArchitectureVmIDPrefix                       = "architecture-"
	OvirtOriginVmID                              = "ovirt-origin"
	PlacementPolicyAffinityVmIDPrefix            = "placement-policy-affinity"
	UsbDisabledVmID                              = "usb-disabled"
	I6300esbWatchdogVmID                         = "i6300esb-watchdog"
	CPUPinningVmID                               = "cpu-pinning"
	UpStatusVmID                                 = "status-up"
	MultipleVmsNo1VmID                           = "multiple-vms-no1"
	MultipleVmsNo2VmID                           = "multiple-vms-no2"

	MissingOVirtSecretVmId   = "missing-ovirt-secret"
	InvalidOVirtSecretVmId   = "invalid-ovirt-secret"
	InvalidOVirtUrlVmID      = "invalid-ovirt-url"
	InvalidOVirtUsernameVmID = "invalid-ovirt-username"
	InvalidOVirtPasswordVmID = "invalid-ovirt-password"
	InvalidOVirtCACertVmID   = "invalid-ovirt-ca-cert"

	MissingExternalResourceMappingVmID = "missing-external-resource-mapping"
)

var (
	DiskID          = "disk-1"
	StorageDomainID = "domain-1"

	VNicProfile1ID = "vnic-profile-1"
	VNicProfile2ID = "vnic-profile-2"

	BasicNetworkVmNicMAC = "56:6f:05:0f:00:05"
	Nic2MAC              = "56:6f:05:0f:00:06"
)
