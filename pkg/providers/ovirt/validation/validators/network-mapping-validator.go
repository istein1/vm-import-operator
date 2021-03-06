package validators

import (
	"encoding/json"
	"fmt"
	"strings"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	outils "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/utils"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

//NetworkAttachmentDefinitionProvider provides NetworkAttachmentDefinition retrieval capabilities
type NetworkAttachmentDefinitionProvider interface {
	//Find retrieves NetworkAttachmentDefinition for given name and optional namespace
	Find(name string, namespace string) (*netv1.NetworkAttachmentDefinition, error)
}

//NetworkMappingValidator provides network mappings validation logic
type NetworkMappingValidator struct {
	provider NetworkAttachmentDefinitionProvider
}

//NewNetworkMappingValidator creates new NetworkMappingValidator that will use given provider
func NewNetworkMappingValidator(provider NetworkAttachmentDefinitionProvider) NetworkMappingValidator {
	return NetworkMappingValidator{
		provider: provider,
	}
}

//ValidateNetworkMapping validates network mapping
func (v *NetworkMappingValidator) ValidateNetworkMapping(nics []*ovirtsdk.Nic, mapping *[]v2vv1.NetworkResourceMappingItem, crNamespace string) []ValidationFailure {
	var failures []ValidationFailure
	// Check whether mapping for network is required and was provided
	if mapping == nil {
		if v.hasAtLeastOneWithVNicProfile(nics) {
			failures = append(failures, ValidationFailure{
				ID:      NetworkMappingID,
				Message: "Network mapping is missing",
			})
		}
		return failures
	}

	// Validate non-duplicates in source networks mapping for both Name and ID
	usedNames := make(map[string]bool)
	usedIDs := make(map[string]bool)
	for _, mapp := range *mapping {
		if mapp.Source.ID != nil {
			if val, _ := usedIDs[*mapp.Source.ID]; val {
				failures = append(failures, ValidationFailure{
					ID:      NetworkSourceDuplicateID,
					Message: fmt.Sprintf("There are duplicate source network entries for ID: %v ", *mapp.Source.Name),
				})
			}
			usedIDs[*mapp.Source.ID] = true
		}
		if mapp.Source.Name != nil {
			if val, _ := usedNames[*mapp.Source.Name]; val {
				failures = append(failures, ValidationFailure{
					ID:      NetworkSourceDuplicateID,
					Message: fmt.Sprintf("There are duplicate source network entries for name: %v ", *mapp.Source.Name),
				})
			}
			usedNames[*mapp.Source.Name] = true
		}
	}

	// Map source id and name to ResourceMappingItem
	mapByID, mapByName := utils.IndexNetworkByIDAndName(mapping)

	// validate source network format comply to network-name/vnic-profile-name
	failure, ok := v.validateSourceNetworkFormat(mapByName)
	if !ok {
		failures = append(failures, failure)
		return failures
	}

	// Get all vnic profiles needed by the VM as slice of sources
	requiredVnicProfiles := v.getRequiredVnicProfiles(nics)

	requiredTargetsSet := make(map[v2vv1.ObjectIdentifier]*string)
	// Validate that all vm networks are mapped and populate requiredTargetsSet for target existence check
	for _, vnic := range requiredVnicProfiles {
		if vnic.ID != nil {
			item, found := mapByID[*vnic.ID]
			if found {
				requiredTargetsSet[item.Target] = item.Type
				continue
			}
		}
		if vnic.Name != nil {
			item, found := mapByName[*vnic.Name]
			if found {
				requiredTargetsSet[item.Target] = item.Type
				continue
			}
		}
		failures = append(failures, ValidationFailure{
			ID:      NetworkMappingID,
			Message: fmt.Sprintf("Required source Vnic Profile '%s' lacks mapping", utils.ToLoggableID(vnic.ID, vnic.Name)),
		})
	}

	podNetworks := v.getPodNetworks(nics, mapByID, mapByName)
	if len(podNetworks) > 1 {
		failures = append(failures, ValidationFailure{
			ID:      NetworkMultiplePodTargetsID,
			Message: fmt.Sprintf("There are more than one source networks mapped to a pod network: %v ", podNetworks),
		})
	}

	// Validate that all target networks needed by the VM exist in k8s
	for networkID, networkType := range requiredTargetsSet {
		if failure, valid := v.validateNetwork(networkID, networkType, crNamespace); !valid {
			failures = append(failures, failure)
		}
	}

	// Validate that source and target are sroiv when used
	if fls, valid := v.validateSRIOV(nics, mapping, crNamespace); !valid {
		failures = append(failures, fls...)
	}
	return failures
}

func (v *NetworkMappingValidator) validateSRIOV(nics []*ovirtsdk.Nic, mapping *[]v2vv1.NetworkResourceMappingItem, crNamespace string) ([]ValidationFailure, bool) {
	var failures []ValidationFailure
	valid := true
	for _, nic := range nics {
		if vnicProfile, ok := nic.VnicProfile(); ok {
			if outils.IsSRIOV(vnicProfile) {
				name, ns := getSROIVNetworkNameForNic(vnicProfile, mapping, crNamespace)
				network, err := v.provider.Find(name, ns)
				if err != nil {
					failures = append(failures, ValidationFailure{
						ID:      NetworkTargetID,
						Message: fmt.Sprintf("Network Attachment Defintion %s has not been found. Error: %v", utils.ToLoggableResourceName(name, &ns), err),
					})
					valid = false
					continue
				}
				var cnf map[string]interface{}
				err = json.Unmarshal([]byte(network.Spec.Config), &cnf)
				if err != nil {
					failures = append(failures, ValidationFailure{
						ID:      NetworkConfig,
						Message: fmt.Sprintf("Network Attachment Defintion %s has not correct config. Error: %v", utils.ToLoggableResourceName(name, &ns), err),
					})
					valid = false
					continue
				}
				if cnf["type"].(string) != "sriov" {
					failures = append(failures, ValidationFailure{
						ID:      NetworkTypeID,
						Message: fmt.Sprintf("Network Attachment Defintion %s is not SRIOV network. Error: %v", utils.ToLoggableResourceName(name, &ns), err),
					})
					valid = false
					continue
				}
			}
		}
	}
	return failures, valid
}

func getSROIVNetworkNameForNic(vnicProfile *ovirtsdk.VnicProfile, mappings *[]v2vv1.NetworkResourceMappingItem, crNamespace string) (string, string) {
	network, _ := vnicProfile.Network()
	nicNetworkName, _ := network.Name()
	vnicProfileName, _ := vnicProfile.Name()

	nicMappingName := outils.GetNetworkMappingName(nicNetworkName, vnicProfileName)
	for _, mapping := range *mappings {
		if mapping.Source.Name != nil && nicMappingName == *mapping.Source.Name {
			return mapNetworkType(mapping, crNamespace)
		}
		if mapping.Source.ID != nil {
			if vnicProfileID, _ := vnicProfile.Id(); vnicProfileID == *mapping.Source.ID {
				return mapNetworkType(mapping, crNamespace)
			}
		}
	}
	return "", ""
}

func mapNetworkType(mapping v2vv1.NetworkResourceMappingItem, crNamespace string) (string, string) {
	namespace := crNamespace
	if mapping.Target.Namespace != nil {
		namespace = *mapping.Target.Namespace
	}
	name := mapping.Target.Name
	return name, namespace
}

func (v *NetworkMappingValidator) getPodNetworks(nics []*ovirtsdk.Nic, mapByID map[string]v2vv1.NetworkResourceMappingItem, mapByName map[string]v2vv1.NetworkResourceMappingItem) []string {
	var podNetworks []string
	for _, nic := range nics {
		if vnicProfile, ok := nic.VnicProfile(); ok {
			if id, ok := vnicProfile.Id(); ok {
				if item, found := mapByID[id]; found {
					if item.Type == nil || *item.Type == "pod" {
						podNetworks = append(podNetworks, utils.ToLoggableID(item.Source.ID, item.Source.Name))
						continue
					}
				}
			}
			if name, found := vnicProfile.Name(); found {
				if item, ok := mapByName[name]; ok {
					if item.Type == nil || *item.Type == "pod" {
						podNetworks = append(podNetworks, utils.ToLoggableID(item.Source.ID, item.Source.Name))
					}
				}
			}
		}
	}
	return podNetworks
}

func (v *NetworkMappingValidator) hasAtLeastOneWithVNicProfile(nics []*ovirtsdk.Nic) bool {
	for _, nic := range nics {
		if _, ok := nic.VnicProfile(); ok {
			return true
		}
	}
	return false
}

func (v *NetworkMappingValidator) validateSourceNetworkFormat(mapByName map[string]v2vv1.NetworkResourceMappingItem) (ValidationFailure, bool) {
	invalidNames := make([]string, 0)
	for k := range mapByName {
		if !strings.Contains(k, "/") {
			invalidNames = append(invalidNames, k)
		}
	}
	if len(invalidNames) > 0 {
		message := fmt.Sprintf("Network mapping name format is invalid: %v. Expected format is 'network-name/vnic-profile-name'", invalidNames)
		return ValidationFailure{
			ID:      NetworkMappingID,
			Message: message,
		}, false
	}

	return ValidationFailure{}, true
}

func (v *NetworkMappingValidator) validateNetwork(networkID v2vv1.ObjectIdentifier, networkType *string, crNamespace string) (ValidationFailure, bool) {
	if networkType == nil {
		if networkID.Namespace == nil {
			// unspecified network type is only valid when target network doesn't specify namespace
			return ValidationFailure{}, true
		}
		return ValidationFailure{
			ID:      NetworkTypeID,
			Message: fmt.Sprintf("Network %s has unspecified network type and target namespace: %s. When target type is omitted, the namespace must be omitted as well.", utils.ToLoggableResourceName(networkID.Name, networkID.Namespace), *networkID.Namespace),
		}, false
	}

	switch *networkType {
	case "pod":
		return ValidationFailure{}, true
	case "multus":
		return v.isValidMultusNetwork(networkID, crNamespace)
	default:
		return ValidationFailure{
			ID:      NetworkTypeID,
			Message: fmt.Sprintf("Network %s has unsupported network type: %v", utils.ToLoggableResourceName(networkID.Name, networkID.Namespace), networkType),
		}, false
	}
}

func (v *NetworkMappingValidator) isValidMultusNetwork(networkID v2vv1.ObjectIdentifier, crNamespace string) (ValidationFailure, bool) {
	namespace := crNamespace
	if networkID.Namespace != nil {
		namespace = *networkID.Namespace
	}
	_, err := v.provider.Find(networkID.Name, namespace)
	if err != nil {
		return ValidationFailure{
			ID:      NetworkTargetID,
			Message: fmt.Sprintf("Network Attachment Defintion %s has not been found. Error: %v", utils.ToLoggableResourceName(networkID.Name, &namespace), err),
		}, false
	}

	return ValidationFailure{}, true
}

func (v *NetworkMappingValidator) getRequiredVnicProfiles(nics []*ovirtsdk.Nic) []v2vv1.Source {
	sourcesSet := make(map[v2vv1.Source]bool)
	for _, nic := range nics {
		if vnic, ok := nic.VnicProfile(); ok {
			if network, ok := vnic.Network(); ok {
				if src, ok := v.createSourceNetworkIdentifier(network, vnic); ok {
					sourcesSet[*src] = true
				}
			}
		}
	}
	var sources []v2vv1.Source
	for source := range sourcesSet {
		sources = append(sources, source)
	}
	return sources
}

func (v *NetworkMappingValidator) createSourceNetworkIdentifier(network *ovirtsdk.Network, vnic *ovirtsdk.VnicProfile) (*v2vv1.Source, bool) {
	id, okID := vnic.Id()
	networkName, okNetworkName := network.Name()
	vnicName, okVnicName := vnic.Name()
	if okID || okNetworkName && okVnicName {
		name := outils.GetNetworkMappingName(networkName, vnicName)
		src := v2vv1.Source{
			ID:   &id,
			Name: &name}
		return &src, true
	}
	return nil, false
}
