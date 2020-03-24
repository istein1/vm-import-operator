package validation

import (
	"context"
	"fmt"
	"time"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"

	validators "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovirtsdk "github.com/ovirt/go-ovirt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type action int

var logger = logf.Log.WithName("validation")

const (
	log   = 0
	warn  = 1
	block = 2

	warnReason  = string(v2vv1alpha1.MappingRulesCheckingReportedWarnings)
	errorReason = string(v2vv1alpha1.MappingRulesCheckingFailed)
	okReason    = string(v2vv1alpha1.MappingRulesCheckingCompleted)
)

var checkToAction = map[validators.CheckID]action{
	// NIC rules
	validators.NicInterfaceCheckID:       block,
	validators.NicOnBootID:               log,
	validators.NicPluggedID:              warn,
	validators.NicVNicPassThroughID:      block,
	validators.NicVNicPortMirroringID:    warn,
	validators.NicVNicCustomPropertiesID: warn,
	validators.NicVNicNetworkFilterID:    warn,
	validators.NicVNicQosID:              log,
	// Storage rules
	validators.DiskAttachmentInterfaceID:           block,
	validators.DiskAttachmentLogicalNameID:         log,
	validators.DiskAttachmentPassDiscardID:         log,
	validators.DiskAttachmentUsesScsiReservationID: block,
	validators.DiskInterfaceID:                     block,
	validators.DiskLogicalNameID:                   log,
	validators.DiskUsesScsiReservationID:           block,
	validators.DiskBackupID:                        warn,
	validators.DiskLunStorageID:                    block,
	validators.DiskPropagateErrorsID:               log,
	validators.DiskWipeAfterDeleteID:               log,
	validators.DiskStatusID:                        block,
	validators.DiskStoragaTypeID:                   block,
	validators.DiskSgioID:                          block,
	// VM rules
	validators.VMBiosBootMenuID:                  log,
	validators.VMBiosTypeID:                      block,
	validators.VMBiosTypeQ35SecureBootID:         warn,
	validators.VMCpuArchitectureID:               block,
	validators.VMCpuTuneID:                       warn,
	validators.VMCpuSharesID:                     log,
	validators.VMCustomPropertiesID:              warn,
	validators.VMDisplayTypeID:                   log,
	validators.VMHasIllegalImagesID:              block,
	validators.VMHighAvailabilityPriorityID:      log,
	validators.VMIoThreadsID:                     warn,
	validators.VMMemoryPolicyBallooningID:        log,
	validators.VMMemoryPolicyOvercommitPercentID: log,
	validators.VMMemoryPolicyGuaranteedID:        log,
	validators.VMMigrationID:                     log,
	validators.VMMigrationDowntimeID:             log,
	validators.VMNumaTuneModeID:                  warn,
	validators.VMOriginID:                        block,
	validators.VMRngDeviceSourceID:               log,
	validators.VMSoundcardEnabledID:              warn,
	validators.VMStartPausedID:                   log,
	validators.VMStorageErrorResumeBehaviourID:   log,
	validators.VMTunnelMigrationID:               warn,
	validators.VMUsbID:                           block,
	validators.VMGraphicConsolesID:               log,
	validators.VMHostDevicesID:                   log,
	validators.VMReportedDevicesID:               log,
	validators.VMQuotaID:                         log,
	validators.VMWatchdogsID:                     block,
	validators.VMCdromsID:                        log,
	validators.VMFloppiesID:                      log,
}

// Validator validates different properties of a VM
type Validator interface {
	ValidateVM(vm *ovirtsdk.Vm) []validators.ValidationFailure
	ValidateDiskAttachments(diskAttachments []*ovirtsdk.DiskAttachment) []validators.ValidationFailure
	ValidateNics(nics []*ovirtsdk.Nic) []validators.ValidationFailure
}

// VirtualMachineImportValidator validates VirtualMachineImport object
type VirtualMachineImportValidator struct {
	Validator Validator
	client    client.Client
}

// NewVirtualMachineImportValidator creates ready-to-use NewVirtualMachineImportValidator
func NewVirtualMachineImportValidator(client client.Client) VirtualMachineImportValidator {
	return VirtualMachineImportValidator{
		Validator: &validators.ValidatorWrapper{},
		client:    client,
	}
}

// Validate validates whether VM described in VirtualMachineImport can be imported
func (validator *VirtualMachineImportValidator) Validate(vm *ovirtsdk.Vm, vmiCrName *types.NamespacedName) error {
	failures := validator.Validator.ValidateVM(vm)
	if nics, ok := vm.Nics(); ok {
		failures = append(failures, validator.Validator.ValidateNics(nics.Slice())...)
	}
	if das, ok := vm.DiskAttachments(); ok {
		failures = append(failures, validator.Validator.ValidateDiskAttachments(das.Slice())...)
	}

	return validator.processFailures(failures, vmiCrName)
}

func (validator *VirtualMachineImportValidator) processFailures(failures []validators.ValidationFailure, vmiCrName *types.NamespacedName) error {
	valid := true
	var warnMessage, errorMessage string

	for _, failure := range failures {
		switch checkToAction[failure.ID] {
		case log:
			logger.Info(fmt.Sprintf("Validation information for %v: %v", vmiCrName, failure))
		case warn:
			warnMessage = withMessage(warnMessage, failure.Message)
		case block:
			valid = false
			errorMessage = withMessage(errorMessage, failure.Message)
		}
	}

	instance := &v2vv1alpha1.VirtualMachineImport{}
	err := validator.client.Get(context.TODO(), *vmiCrName, instance)
	if err != nil {
		return err
	}
	copy := instance.DeepCopy()

	if !valid {
		updateCondition(&copy.Status.Conditions, errorReason, errorMessage, false)
	} else if warnMessage != "" {
		updateCondition(&copy.Status.Conditions, warnReason, warnMessage, true)
	} else {
		updateCondition(&copy.Status.Conditions, okReason, "All mapping rules checks passed", true)
	}

	err = validator.client.Status().Update(context.TODO(), copy)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("Validation failed for %v. Reasons: %s", vmiCrName, errorMessage)
	}
	return nil
}

func withMessage(message string, newMessage string) string {
	if message == "" {
		return newMessage
	}
	return fmt.Sprintf("%s, %s", message, newMessage)
}

func updateCondition(conditions *[]v2vv1alpha1.VirtualMachineImportCondition, reason string, message string, status bool) {
	conditionStatus := v1.ConditionTrue
	if !status {
		conditionStatus = v1.ConditionFalse
	}
	conditionType := v2vv1alpha1.MappingRulesChecking

	condition := findConditionOfType(conditionType, *conditions)
	now := metav1.NewTime(time.Now())

	if condition != nil {
		condition.Message = &message
		condition.Reason = &reason
		condition.LastHeartbeatTime = &now
		if condition.Status != conditionStatus {
			condition.Status = conditionStatus
			condition.LastTransitionTime = &now
		}
	} else {
		newCondition := v2vv1alpha1.VirtualMachineImportCondition{
			Type:               conditionType,
			LastTransitionTime: &now,
			LastHeartbeatTime:  &now,
			Message:            &message,
			Reason:             &reason,
			Status:             conditionStatus,
		}
		*conditions = append(*conditions, newCondition)
	}
}

func findConditionOfType(tp v2vv1alpha1.VirtualMachineImportConditionType, conditions []v2vv1alpha1.VirtualMachineImportCondition) *v2vv1alpha1.VirtualMachineImportCondition {
	for i := range conditions {
		if conditions[i].Type == tp {
			return &conditions[i]
		}
	}
	return nil
}