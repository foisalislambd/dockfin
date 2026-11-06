package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dockfin/dockfin/internal/imagescan"
	"github.com/dockfin/dockfin/internal/sshdial"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func (q *Queue) scanDeployedImages(ctx context.Context, teamID, serverID, deploymentID, resourceID uuid.UUID, kind string, refs []string, log func(stage, line string)) error {
	policy, err := q.Store.ImageScanPolicy(ctx)
	if err != nil || !policy.Enabled || len(refs) == 0 {
		return nil
	}
	client, err := sshdial.DialClient(ctx, q.Store, q.SSH, teamID, serverID)
	if err != nil {
		if policy.Gate == "block" {
			return fmt.Errorf("image scan: %w", err)
		}
		if log != nil {
			log("scan", "image scan skipped: "+err.Error())
		}
		return nil
	}
	ignored, err := q.Store.ImageScanIgnores(ctx, teamID)
	if err != nil {
		return err
	}
	var blocked string
	for _, ref := range refs {
		if !imagescan.ValidImageRef(ref) {
			continue
		}
		if log != nil {
			log("scan", "Scanning "+ref)
		}
		raw, runErr := imagescan.Run(client, ref)
		scan := store.ImageScan{
			TeamID: teamID, ResourceType: kind, ResourceID: resourceID, DeploymentID: &deploymentID,
			ServerID: &serverID, ImageRef: ref, Status: "completed", Findings: []byte("[]"),
		}
		if runErr != nil {
			scan.Status = "failed"
			scan.Error = runErr.Error()
			if policy.Gate == "block" {
				blocked = runErr.Error()
			} else if log != nil {
				log("scan", "image scan failed: "+runErr.Error())
			}
		} else {
			sum, perr := imagescan.ParseGrype(raw, ignored)
			if perr != nil {
				scan.Status = "failed"
				scan.Error = perr.Error()
			} else {
				scan.Critical, scan.High, scan.Medium, scan.Low, scan.Unknown = sum.Critical, sum.High, sum.Medium, sum.Low, sum.Unknown
				b, _ := json.Marshal(sum.Findings)
				scan.Findings = b
				if log != nil {
					log("scan", fmt.Sprintf("%s: %d critical, %d high, %d medium, %d low", ref, sum.Critical, sum.High, sum.Medium, sum.Low))
				}
				if policy.Gate == "block" && sum.Critical > 0 {
					blocked = fmt.Sprintf("%s has %d critical vulnerabilities", ref, sum.Critical)
				}
			}
		}
		_, _ = q.Store.InsertImageScan(ctx, scan)
	}
	if blocked != "" {
		return fmt.Errorf("image scan blocked the deploy: %s", blocked)
	}
	return nil
}
