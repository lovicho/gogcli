package cmd

import (
	"fmt"
	"sort"
	"strings"
)

type driveDuSummary struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Files int    `json:"files"`
	Depth int    `json:"depth"`
}

func summarizeDriveDu(items []driveTreeItem, rootID string, depthLimit int) ([]driveDuSummary, error) {
	type folderMeta struct {
		id              string
		path            string
		depth           int
		parentPlacement drivePlacementID
	}

	folderMetaByPlacement := map[drivePlacementID]folderMeta{
		driveRootPlacementID: {
			id:   rootID,
			path: ".",
		},
	}
	for _, it := range items {
		if it.IsFolder() {
			if it.placementID == 0 {
				return nil, fmt.Errorf("drive folder %q is missing placement identity", it.ID)
			}
			if _, exists := folderMetaByPlacement[it.placementID]; exists {
				return nil, fmt.Errorf("duplicate drive placement identity %d", it.placementID)
			}
			folderMetaByPlacement[it.placementID] = folderMeta{
				id:              it.ID,
				path:            it.Path,
				depth:           it.Depth,
				parentPlacement: it.parentPlacementID,
			}
		}
	}

	sizes := map[drivePlacementID]*driveDuSummary{}
	getSummary := func(placementID drivePlacementID) *driveDuSummary {
		if s, ok := sizes[placementID]; ok {
			return s
		}
		meta := folderMetaByPlacement[placementID]
		s := &driveDuSummary{
			ID:    meta.id,
			Path:  meta.path,
			Depth: meta.depth,
		}
		sizes[placementID] = s
		return s
	}

	for _, it := range items {
		if it.IsFolder() {
			continue
		}
		if it.parentPlacementID == 0 {
			return nil, fmt.Errorf("drive item %q is missing parent placement identity", it.ID)
		}
		parentPlacement := it.parentPlacementID
		visited := make(map[drivePlacementID]struct{}, it.Depth+1)
		for parentPlacement != 0 {
			if _, exists := visited[parentPlacement]; exists {
				return nil, fmt.Errorf("drive placement ancestry cycle at %d", parentPlacement)
			}
			visited[parentPlacement] = struct{}{}
			meta, ok := folderMetaByPlacement[parentPlacement]
			if !ok {
				return nil, fmt.Errorf("drive item %q references unknown parent placement %d", it.ID, parentPlacement)
			}
			s := getSummary(parentPlacement)
			s.Size += it.Size
			s.Files++
			parentPlacement = meta.parentPlacement
		}
	}

	out := make([]driveDuSummary, 0, len(sizes))
	for _, s := range sizes {
		if depthLimit > 0 && s.Depth > depthLimit {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

func sortDriveDu(items []driveDuSummary, sortBy string, order string) {
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))
	order = strings.ToLower(strings.TrimSpace(order))
	desc := order == "desc"

	less := func(i, j int) bool { return false }
	switch sortBy {
	case "path":
		less = func(i, j int) bool { return items[i].Path < items[j].Path }
	case "files":
		less = func(i, j int) bool { return items[i].Files < items[j].Files }
	default:
		less = func(i, j int) bool { return items[i].Size < items[j].Size }
	}

	sort.Slice(items, func(i, j int) bool {
		if desc {
			return less(j, i)
		}
		return less(i, j)
	})
}

func sortDriveInventory(items []driveTreeItem, sortBy string, order string) {
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))
	order = strings.ToLower(strings.TrimSpace(order))
	desc := order == "desc"

	less := func(i, j int) bool { return false }
	switch sortBy {
	case "size":
		less = func(i, j int) bool { return items[i].Size < items[j].Size }
	case "modified":
		less = func(i, j int) bool { return items[i].ModifiedTime < items[j].ModifiedTime }
	default:
		less = func(i, j int) bool { return items[i].Path < items[j].Path }
	}

	sort.Slice(items, func(i, j int) bool {
		if desc {
			return less(j, i)
		}
		return less(i, j)
	})
}
