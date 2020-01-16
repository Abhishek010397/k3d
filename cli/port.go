package run

import (
	"fmt"
	"strings"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

// mapNodesToPortSpecs maps nodes to portSpecs
func mapNodesToPortSpecs(specs []string, createdNodes []string) (map[string][]string, error) {

	if err := validatePortSpecs(specs); err != nil {
		return nil, err
	}

	// check node-specifier possibilitites
	possibleNodeSpecifiers := []string{"all", "workers", "agents", "server", "master"}
	possibleNodeSpecifiers = append(possibleNodeSpecifiers, createdNodes...)

	nodeToPortSpecMap := make(map[string][]string)

	for _, spec := range specs {
		nodes, portSpec := extractNodes(spec)

		if len(nodes) == 0 {
			nodes = append(nodes, defaultNodes)
		}

		for _, node := range nodes {
			// check if node-specifier is valid (either a role or a name) and append to list if matches
			nodeFound := false
			for _, name := range possibleNodeSpecifiers {
				if node == name {
					nodeFound = true
					nodeToPortSpecMap[node] = append(nodeToPortSpecMap[node], portSpec)
					break
				}
			}
			if !nodeFound {
				log.Warningf("Unknown node-specifier [%s] in port mapping entry [%s]", node, spec)
			}
		}
	}

	return nodeToPortSpecMap, nil
}

// CreatePublishedPorts is the factory function for PublishedPorts
func CreatePublishedPorts(specs []string) (*PublishedPorts, error) {
	if len(specs) == 0 {
		var newExposedPorts = make(map[nat.Port]struct{}, 1)
		var newPortBindings = make(map[nat.Port][]nat.PortBinding, 1)
		return &PublishedPorts{ExposedPorts: newExposedPorts, PortBindings: newPortBindings}, nil
	}

	newExposedPorts, newPortBindings, err := nat.ParsePortSpecs(specs)
	return &PublishedPorts{ExposedPorts: newExposedPorts, PortBindings: newPortBindings}, err
}

// validatePortSpecs matches the provided port specs against a set of rules to enable early exit if something is wrong
func validatePortSpecs(specs []string) error {
	for _, spec := range specs {
		atSplit := strings.Split(spec, "@")
		_, err := nat.ParsePortSpec(atSplit[0])
		if err != nil {
			return fmt.Errorf("Invalid port specification [%s] in port mapping [%s]\n%+v", atSplit[0], spec, err)
		}
		if len(atSplit) > 0 {
			for i := 1; i < len(atSplit); i++ {
				if err := ValidateHostname(atSplit[i]); err != nil {
					return fmt.Errorf("Invalid node-specifier [%s] in port mapping [%s]\n%+v", atSplit[i], spec, err)
				}
			}
		}
	}
	return nil
}

// extractNodes separates the node specification from the actual port specs
func extractNodes(spec string) ([]string, string) {
	// extract nodes
	nodes := []string{}
	atSplit := strings.Split(spec, "@")
	portSpec := atSplit[0]
	if len(atSplit) > 1 {
		nodes = atSplit[1:]
	}
	if len(nodes) == 0 {
		nodes = append(nodes, defaultNodes)
	}
	return nodes, portSpec
}

// Offset creates a new PublishedPort structure, with all host ports are changed by a fixed 'offset'
func (p PublishedPorts) Offset(offset int) *PublishedPorts {
	var newExposedPorts = make(map[nat.Port]struct{}, len(p.ExposedPorts))
	var newPortBindings = make(map[nat.Port][]nat.PortBinding, len(p.PortBindings))

	for k, v := range p.ExposedPorts {
		newExposedPorts[k] = v
	}

	for k, v := range p.PortBindings {
		bindings := make([]nat.PortBinding, len(v))
		for i, b := range v {
			port, _ := nat.ParsePort(b.HostPort)
			bindings[i].HostIP = b.HostIP
			bindings[i].HostPort = fmt.Sprintf("%d", port*offset)
		}
		newPortBindings[k] = bindings
	}

	return &PublishedPorts{ExposedPorts: newExposedPorts, PortBindings: newPortBindings}
}

// AddPort creates a new PublishedPort struct with one more port, based on 'portSpec'
func (p *PublishedPorts) AddPort(portSpec string) (*PublishedPorts, error) {
	portMappings, err := nat.ParsePortSpec(portSpec)
	if err != nil {
		return nil, err
	}

	var newExposedPorts = make(map[nat.Port]struct{}, len(p.ExposedPorts)+1)
	var newPortBindings = make(map[nat.Port][]nat.PortBinding, len(p.PortBindings)+1)

	// Populate the new maps
	for k, v := range p.ExposedPorts {
		newExposedPorts[k] = v
	}

	for k, v := range p.PortBindings {
		newPortBindings[k] = v
	}

	// Add new ports
	for _, portMapping := range portMappings {
		port := portMapping.Port
		if _, exists := newExposedPorts[port]; !exists {
			newExposedPorts[port] = struct{}{}
		}

		bslice, exists := newPortBindings[port]
		if !exists {
			bslice = []nat.PortBinding{}
		}
		newPortBindings[port] = append(bslice, portMapping.Binding)
	}

	return &PublishedPorts{ExposedPorts: newExposedPorts, PortBindings: newPortBindings}, nil
}

// MergePortSpecs merges published ports for a given node
func MergePortSpecs(nodeToPortSpecMap map[string][]string, role, name string) ([]string, error) {

	portSpecs := []string{}

	// add portSpecs according to node role
	for _, group := range nodeRuleGroupsMap[role] {
		for _, v := range nodeToPortSpecMap[group] {
			exists := false
			for _, i := range portSpecs {
				if v == i {
					exists = true
				}
			}
			if !exists {
				portSpecs = append(portSpecs, v)
			}
		}
	}

	// add portSpecs according to node name
	for _, v := range nodeToPortSpecMap[name] {
		exists := false
		for _, i := range portSpecs {
			if v == i {
				exists = true
			}
		}
		if !exists {
			portSpecs = append(portSpecs, v)
		}
	}

	return portSpecs, nil
}
