package sshdial

import (
	"context"
	"fmt"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

const maxJumpHops = 4

// Dial opens an SSH client to serverID, walking jump_host_id if set.
func Dial(ctx context.Context, st *store.Store, pool *sshx.Pool, teamID, serverID uuid.UUID) (*sshx.DialResult, error) {
	return dial(ctx, st, pool, teamID, serverID, map[uuid.UUID]struct{}{})
}

func DialClient(ctx context.Context, st *store.Store, pool *sshx.Pool, teamID, serverID uuid.UUID) (*ssh.Client, error) {
	res, err := Dial(ctx, st, pool, teamID, serverID)
	if err != nil {
		return nil, err
	}
	return res.Client, nil
}

func dial(ctx context.Context, st *store.Store, pool *sshx.Pool, teamID, serverID uuid.UUID, seen map[uuid.UUID]struct{}) (*sshx.DialResult, error) {
	if _, ok := seen[serverID]; ok {
		return nil, fmt.Errorf("jump host cycle detected")
	}
	if len(seen) >= maxJumpHops {
		return nil, fmt.Errorf("too many jump hosts (max %d)", maxJumpHops)
	}
	seen[serverID] = struct{}{}

	srv, err := st.GetServer(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if srv.PrivateKeyID == nil {
		return nil, fmt.Errorf("server has no private key")
	}
	enc, err := st.GetPrivateKeyMaterial(ctx, teamID, *srv.PrivateKeyID)
	if err != nil {
		return nil, err
	}
	priv, err := st.Box.DecryptString(enc)
	if err != nil {
		return nil, err
	}

	var jump *ssh.Client
	if srv.JumpHostID != nil {
		jres, err := dial(ctx, st, pool, teamID, *srv.JumpHostID, seen)
		if err != nil {
			return nil, fmt.Errorf("jump host: %w", err)
		}
		jump = jres.Client
	}

	if pool == nil {
		return nil, fmt.Errorf("ssh pool unavailable")
	}
	res, err := pool.Dial(sshx.Target{
		Host:                srv.IP,
		Port:                srv.Port,
		User:                srv.UserName,
		PrivateKey:          []byte(priv),
		ExpectedFingerprint: srv.HostKeyFingerprint,
		ExpectedKeyType:     srv.HostKeyType,
		Jump:                jump,
	})
	if err != nil {
		return nil, err
	}
	if res.IsNewHost && res.Fingerprint != "" {
		_ = st.UpdateServerHostKey(ctx, serverID, res.Fingerprint, res.KeyType)
	}
	return res, nil
}
