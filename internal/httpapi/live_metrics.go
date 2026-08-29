package httpapi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dockfin/dockfin/internal/sshdial"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

const liveMetricsTTL = 28 * time.Second

var (
	liveFreshMu sync.Mutex
	liveFreshAt = map[uuid.UUID]time.Time{}
	liveSampleMu sync.Mutex
	liveSampling = map[uuid.UUID]bool{}
)

// hostSampleScript gathers CPU (1s delta), memory, and root disk. One SSH exec.
const hostSampleScript = `set -eu
PROC=/proc
read_cpu() { awk '/^cpu / {print $2+$4, $2+$4+$5}' "$PROC/stat"; }
set -- $(read_cpu); U1=$1 T1=$2
sleep 1
set -- $(read_cpu); U2=$1 T2=$2
DU=$((U2-U1)); DT=$((T2-T1))
if [ "$DT" -gt 0 ]; then CPU=$(awk -v du="$DU" -v dt="$DT" 'BEGIN { printf "%.2f", 100*du/dt }'); else CPU=0; fi
MEM_TOTAL=$(awk '/MemTotal/ {print $2*1024}' "$PROC/meminfo")
MEM_AVAIL=$(awk '/MemAvailable/ {print $2*1024}' "$PROC/meminfo")
MEM_USED=$((MEM_TOTAL-MEM_AVAIL))
DISK_TOTAL=$(df -B1 / 2>/dev/null | awk 'NR==2 {print $2}')
DISK_USED=$(df -B1 / 2>/dev/null | awk 'NR==2 {print $3}')
: "${DISK_TOTAL:=0}"
: "${DISK_USED:=0}"
printf '%s %s %s %s %s\n' "$CPU" "$MEM_USED" "$MEM_TOTAL" "$DISK_USED" "$DISK_TOTAL"
`

func (a *API) ensureFreshHostMetrics(ctx context.Context, teamID, serverID uuid.UUID) error {
	liveFreshMu.Lock()
	if t, ok := liveFreshAt[serverID]; ok && time.Since(t) < liveMetricsTTL {
		liveFreshMu.Unlock()
		return nil
	}
	liveFreshMu.Unlock()

	recent, err := a.Store.ListServerMetrics(ctx, teamID, serverID, 1)
	if err == nil && len(recent) == 1 && time.Since(recent[0].RecordedAt) < 40*time.Second {
		liveFreshMu.Lock()
		liveFreshAt[serverID] = time.Now()
		liveFreshMu.Unlock()
		return nil
	}

	liveSampleMu.Lock()
	if liveSampling[serverID] {
		liveSampleMu.Unlock()
		return nil
	}
	liveSampling[serverID] = true
	liveSampleMu.Unlock()
	defer func() {
		liveSampleMu.Lock()
		delete(liveSampling, serverID)
		liveSampleMu.Unlock()
	}()

	client, err := sshdialForTeam(ctx, a, teamID, serverID)
	if err != nil {
		return err
	}
	snap, err := sampleHostMetrics(client)
	if err != nil {
		return err
	}
	if err := a.Store.InsertServerMetric(ctx, teamID, serverID, snap); err != nil {
		return err
	}
	liveFreshMu.Lock()
	liveFreshAt[serverID] = time.Now()
	liveFreshMu.Unlock()
	return nil
}

func sshdialForTeam(ctx context.Context, a *API, teamID, serverID uuid.UUID) (*ssh.Client, error) {
	if a.Queue == nil || a.Queue.SSH == nil {
		return nil, fmt.Errorf("ssh pool unavailable")
	}
	return sshdial.DialClient(ctx, a.Store, a.Queue.SSH, teamID, serverID)
}

func sampleHostMetrics(client *ssh.Client) (store.ServerMetric, error) {
	var zero store.ServerMetric
	if client == nil {
		return zero, fmt.Errorf("no ssh client")
	}
	out, errOut, err := sshx.RunArgs(client, "sh", "-c", hostSampleScript)
	if err != nil {
		return zero, fmt.Errorf("host sample: %v %s", err, strings.TrimSpace(errOut))
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 5 {
		return zero, fmt.Errorf("host sample: unexpected output %q", strings.TrimSpace(out))
	}
	cpu, _ := strconv.ParseFloat(fields[0], 64)
	memUsed, _ := strconv.ParseInt(fields[1], 10, 64)
	memTotal, _ := strconv.ParseInt(fields[2], 10, 64)
	diskUsed, _ := strconv.ParseInt(fields[3], 10, 64)
	diskTotal, _ := strconv.ParseInt(fields[4], 10, 64)
	return store.ServerMetric{
		CPUPercent:       cpu,
		MemoryUsedBytes:  memUsed,
		MemoryTotalBytes: memTotal,
		DiskUsedBytes:    diskUsed,
		DiskTotalBytes:   diskTotal,
		RecordedAt:       time.Now().UTC(),
	}, nil
}
