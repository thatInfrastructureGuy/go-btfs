package rm

import (
	"context"
	"fmt"
	"time"

	"github.com/TRON-US/go-btfs/core"
	"github.com/TRON-US/go-btfs/core/commands/cmdenv"

	cmds "github.com/TRON-US/go-btfs-cmds"
	coreiface "github.com/TRON-US/interface-go-btfs-core"
	"github.com/TRON-US/interface-go-btfs-core/options"
	"github.com/TRON-US/interface-go-btfs-core/path"
	ipld "github.com/ipfs/go-ipld-format"
)

func RmDag(ctx context.Context, hashes []string, n *core.IpfsNode, req *cmds.Request, env cmds.Environment,
	force bool) ([]string, error) {
	// do not perform online operations for local delete
	api, err := cmdenv.GetApi(env, req, options.Api.Offline(true), options.Api.FetchBlocks(false))
	if err != nil {
		fmt.Println("???", err)
		return nil, err
	}

	var results []string
	for _, b := range hashes {
		// Make sure node exists
		p := path.New(b)
		fmt.Println("resolve p", p, time.Now())
		node, err := api.ResolveNode(ctx, p)
		if err != nil {
			fmt.Println("resolve p err", err, time.Now())
			results = append(results, fmt.Sprintf("Error resolving root %s: %v", b, err))
			continue
		}

		fmt.Println("pinned p", p, time.Now())
		_, pinned, err := n.Pinning.IsPinned(ctx, node.Cid())
		if err != nil {
			fmt.Println("pinned err", err, time.Now())
			return nil, err
		}
		if pinned {
			// Since we are removing a file, we need to set recursive flag to true
			fmt.Println("pin rm p", p, time.Now())
			err = api.Pin().Rm(ctx, p, options.Pin.RmRecursive(true), options.Pin.RmForce(force))
			if err != nil {
				fmt.Println("pin rm err", err, time.Now())
				results = append(results, fmt.Sprintf("Error removing root %s pin: %v", b, err))
				continue
			}
		}

		// Rm all child links
		fmt.Println("dag rm p", p, time.Now())
		err = rmAllDags(ctx, api, node, &results, map[string]bool{})
		if err != nil {
			fmt.Println("dag rm err", err, time.Now())
			results = append(results, fmt.Sprintf("Error removing root %s child objects: %v", b, err))
			continue
		}
	}

	return results, nil
}

func rmAllDags(ctx context.Context, api coreiface.CoreAPI, node ipld.Node, res *[]string,
	removed map[string]bool) error {
	for _, nl := range node.Links() {
		// Skipped just removed nodes
		if _, ok := removed[nl.Cid.String()]; ok {
			continue
		}
		// Resolve, recurse, then finally remove
		rn, err := api.ResolveNode(ctx, path.IpfsPath(nl.Cid))
		if err != nil {
			*res = append(*res, fmt.Sprintf("Error resolving object %s: %v", nl.Cid, err))
			continue // continue with other nodes
		}
		if err := rmAllDags(ctx, api, rn, res, removed); err != nil {
			continue // continue with other nodes
		}
	}
	ncid := node.Cid()
	if err := api.Dag().Remove(ctx, ncid); err != nil {
		*res = append(*res, fmt.Sprintf("Error removing object %s: %v", ncid, err))
		return err
	} else {
		*res = append(*res, fmt.Sprintf("Removed %s", ncid))
		removed[ncid.String()] = true
	}
	return nil
}
