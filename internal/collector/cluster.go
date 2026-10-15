package collector

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

// CRSResource represents a parsed entry from `crsctl status resource -t`.
type CRSResource struct {
	Name   string
	Target string
	State  string
	Node   string
	Detail string
}

// ParseOlsnodes parses output of `olsnodes -n -s -t` (e.g., "ora-rac01 1 Active Unpinned Hub").
func ParseOlsnodes(raw string) []model.ClusterNode {
	var nodes []model.ClusterNode
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		id, _ := strconv.Atoi(fields[1])
		state := "ONLINE"
		if len(fields) >= 3 {
			if strings.EqualFold(fields[2], "Active") {
				state = "ONLINE"
			} else {
				state = strings.ToUpper(fields[2])
			}
		}
		role := "HUB"
		if len(fields) >= 5 {
			role = strings.ToUpper(fields[4])
		} else if len(fields) == 4 {
			role = strings.ToUpper(fields[3])
		}
		nodes = append(nodes, model.ClusterNode{
			ID:           id,
			Name:         name,
			State:        state,
			Role:         role,
			VIP:          "ONLINE",
			VIPState:     "ONLINE",
			Interconnect: "OK",
			CRS:          "ONLINE",
			CSSD:         "ONLINE",
			EVMD:         "ONLINE",
			ASM:          "ONLINE",
		})
	}
	return nodes
}

// ParseCRSCheckAll parses output of `crsctl check cluster -all` to update daemon status per node.
func ParseCRSCheckAll(raw string, nodes []model.ClusterNode) []model.ClusterNode {
	nodeMap := make(map[string]*model.ClusterNode)
	for i := range nodes {
		nodeMap[strings.ToLower(nodes[i].Name)] = &nodes[i]
	}

	var currentNode *model.ClusterNode
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "*") {
			continue
		}
		// Node header line like "ora-rac01:"
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "CRS-") {
			nName := strings.TrimSuffix(trimmed, ":")
			nName = strings.TrimSpace(strings.ToLower(nName))
			if existing, ok := nodeMap[nName]; ok {
				currentNode = existing
			} else {
				nodes = append(nodes, model.ClusterNode{
					ID:    len(nodes) + 1,
					Name:  nName,
					State: "ONLINE",
					Role:  "HUB",
				})
				currentNode = &nodes[len(nodes)-1]
				nodeMap[nName] = currentNode
			}
			continue
		}

		if currentNode == nil && len(nodes) > 0 {
			currentNode = &nodes[0]
		}
		if currentNode == nil {
			continue
		}

		isOnline := strings.Contains(strings.ToLower(trimmed), "online")
		status := "OFFLINE"
		if isOnline {
			status = "ONLINE"
		}
		switch {
		case strings.Contains(trimmed, "CRS-4537") || strings.Contains(trimmed, "Cluster Ready Services"):
			currentNode.CRS = status
		case strings.Contains(trimmed, "CRS-4529") || strings.Contains(trimmed, "Cluster Synchronization Services"):
			currentNode.CSSD = status
		case strings.Contains(trimmed, "CRS-4533") || strings.Contains(trimmed, "Event Manager"):
			currentNode.EVMD = status
		}
	}
	return nodes
}

// ParseCRSStatusResourceT parses `crsctl status resource -t` tabular output.
func ParseCRSStatusResourceT(raw string) []CRSResource {
	var resources []CRSResource
	var currentName string
	inBody := false

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inBody = true
			continue
		}
		if !inBody || strings.HasPrefix(trimmed, "Name ") || strings.HasPrefix(trimmed, "Local Resources") || strings.HasPrefix(trimmed, "Cluster Resources") {
			continue
		}

		// Resource name header line (no spaces, starts with ora.)
		if strings.HasPrefix(trimmed, "ora.") && !strings.Contains(trimmed, " ") {
			currentName = trimmed
			continue
		}

		fields := strings.Fields(trimmed)
		if currentName != "" && len(fields) >= 3 {
			target := fields[0]
			state := fields[1]
			node := fields[2]
			detail := ""
			if len(fields) > 3 {
				detail = strings.Join(fields[3:], " ")
			}
			// Handle numeric instance index in Cluster Resources (e.g. "1 ONLINE ONLINE node1")
			if _, err := strconv.Atoi(fields[0]); err == nil && len(fields) >= 4 {
				target = fields[1]
				state = fields[2]
				node = fields[3]
				if len(fields) > 4 {
					detail = strings.Join(fields[4:], " ")
				}
			}
			resources = append(resources, CRSResource{
				Name:   currentName,
				Target: target,
				State:  state,
				Node:   node,
				Detail: detail,
			})
		}
	}
	return resources
}

// CollectCluster queries Grid Infrastructure binaries to populate cluster nodes and resources.
func CollectCluster(ctx context.Context, runner *executor.Runner, env *discovery.DiscoveredEnvironment) ([]model.ClusterNode, []CRSResource) {
	if env.Topology == model.ModeSingle || env.GridHome == "" {
		return []model.ClusterNode{
			{
				ID:           1,
				Name:         env.HostName,
				State:        "ONLINE",
				Role:         "STANDALONE",
				VIP:          "N/A",
				VIPState:     "N/A",
				Interconnect: "N/A",
				CRS:          "N/A",
				CSSD:         "N/A",
				EVMD:         "N/A",
				ASM:          "N/A",
			},
		}, nil
	}

	gridEnv := map[string]string{"ORACLE_HOME": env.GridHome}
	olsnodesBin := filepath.Join(env.GridHome, "bin", "olsnodes")
	crsctlBin := filepath.Join(env.GridHome, "bin", "crsctl")

	var nodes []model.ClusterNode
	if out, err := runner.RunCommand(ctx, env.GridOwner, false, gridEnv, nil, olsnodesBin, "-n", "-s", "-t"); err == nil && out != "" {
		nodes = ParseOlsnodes(out)
	}
	if len(nodes) == 0 {
		nodes = []model.ClusterNode{{ID: 1, Name: env.HostName, State: "ONLINE", Role: "STANDALONE"}}
	}

	if env.Topology == model.ModeRAC {
		if out, err := runner.RunCommand(ctx, env.GridOwner, false, gridEnv, nil, crsctlBin, "check", "cluster", "-all"); err == nil && out != "" {
			nodes = ParseCRSCheckAll(out, nodes)
		}
	} else {
		if out, err := runner.RunCommand(ctx, env.GridOwner, false, gridEnv, nil, crsctlBin, "check", "has"); err == nil && out != "" {
			status := "OFFLINE"
			if strings.Contains(strings.ToLower(out), "online") {
				status = "ONLINE"
			}
			nodes[0].CRS = status
		}
	}

	var resources []CRSResource
	if out, err := runner.RunCommand(ctx, env.GridOwner, false, gridEnv, nil, crsctlBin, "status", "resource", "-t"); err == nil && out != "" {
		resources = ParseCRSStatusResourceT(out)
		for _, res := range resources {
			for i := range nodes {
				if strings.EqualFold(nodes[i].Name, res.Node) {
					if strings.HasSuffix(res.Name, ".vip") {
						nodes[i].VIP = res.Name + " (" + res.State + ")"
						nodes[i].VIPState = res.State
					} else if strings.HasPrefix(res.Name, "ora.asm") {
						nodes[i].ASM = res.State
					}
				}
			}
		}
	}

	return nodes, resources
}
