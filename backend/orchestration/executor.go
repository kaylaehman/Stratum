package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/nodeconn"
	"github.com/KAE-Labs/stratum/backend/proxmox"
)

// DefaultStepTimeout is the per-step execution deadline.
const DefaultStepTimeout = 60 * time.Second

// ContainerLifecycle is the slice of docker.Client used by the executor.
type ContainerLifecycle interface {
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RestartContainer(ctx context.Context, id string) error
}

// VMLifecycle is the slice of proxmox.Client used by the executor.
type VMLifecycle interface {
	GuestPowerAction(ctx context.Context, node, kind string, vmid int, action string) (string, error)
}

// ConnManager resolves transport clients for a node.
type ConnManager interface {
	Get(ctx context.Context, nodeID string) (*nodeconn.Clients, error)
}

// ExecutorStore is the narrow db interface needed at execution time.
type ExecutorStore interface {
	StoreReader
	GetContainer(ctx context.Context, id string) (db.Container, error)
}

// Executor runs Plans against live infrastructure.
type Executor struct {
	store StoreReader
	conn  ConnManager
}

// NewExecutor constructs an Executor.
func NewExecutor(store StoreReader, conn ConnManager) *Executor {
	return &Executor{store: store, conn: conn}
}

// Execute runs all steps in plan order, stopping on the first failure.
// It returns all results up to and including the failed step.
func (e *Executor) Execute(ctx context.Context, plan Plan) ([]StepResult, error) {
	var results []StepResult
	for _, step := range plan.Steps {
		result := e.runStep(ctx, step)
		results = append(results, result)
		if !result.OK {
			return results, fmt.Errorf("orchestration: step %d (%s %s) failed: %s",
				step.Order, step.Action, step.Name, result.Error)
		}
	}
	return results, nil
}

func (e *Executor) runStep(ctx context.Context, step Step) StepResult {
	start := time.Now()
	stepCtx, cancel := context.WithTimeout(ctx, DefaultStepTimeout)
	defer cancel()

	var err error
	switch step.Kind {
	case "container":
		err = e.execContainer(stepCtx, step)
	case "vm":
		err = e.execVM(stepCtx, step)
	default:
		err = fmt.Errorf("unknown step kind: %s", step.Kind)
	}

	ms := time.Since(start).Milliseconds()
	if err != nil {
		return StepResult{Step: step, OK: false, Error: err.Error(), DurationMs: ms}
	}
	return StepResult{Step: step, OK: true, DurationMs: ms}
}

func (e *Executor) execContainer(ctx context.Context, step Step) error {
	ctr, err := e.resolveContainer(ctx, step.ID)
	if err != nil {
		return err
	}
	clients, err := e.conn.Get(ctx, ctr.NodeID)
	if err != nil || clients.Docker == nil {
		return fmt.Errorf("docker client unavailable for node %s", ctr.NodeID)
	}
	return callDockerLifecycle(ctx, clients.Docker, ctr.DockerID, step.Action)
}

func (e *Executor) execVM(ctx context.Context, step Step) error {
	vm, err := e.resolveVM(ctx, step.ID)
	if err != nil {
		return err
	}
	clients, err := e.conn.Get(ctx, vm.NodeID)
	if err != nil || clients.Proxmox == nil {
		return fmt.Errorf("proxmox client unavailable for node %s", vm.NodeID)
	}
	action := proxmoxAction(step.Action)
	_, err = clients.Proxmox.GuestPowerAction(ctx, vm.ProxmoxNode, vm.Kind, vm.ProxmoxVMID, action)
	return err
}

func callDockerLifecycle(ctx context.Context, c ContainerLifecycle, dockerID, action string) error {
	switch action {
	case "start":
		return c.StartContainer(ctx, dockerID)
	case "stop":
		return c.StopContainer(ctx, dockerID)
	case "restart":
		return c.RestartContainer(ctx, dockerID)
	default:
		return fmt.Errorf("unknown container action: %s", action)
	}
}

func proxmoxAction(action string) string {
	switch action {
	case "stop":
		return "stop"
	case "start":
		return "start"
	case "restart":
		return "reboot"
	default:
		return action
	}
}

// resolveContainer fetches the container row from the store using the container DB id.
func (e *Executor) resolveContainer(ctx context.Context, id string) (db.Container, error) {
	containers, err := e.store.ListNodes(ctx)
	if err != nil {
		return db.Container{}, err
	}
	for _, node := range containers {
		ctrs, err := e.store.ListContainersByNode(ctx, node.ID)
		if err != nil {
			continue
		}
		for _, c := range ctrs {
			if c.ID == id {
				return c, nil
			}
		}
	}
	return db.Container{}, fmt.Errorf("container %s not found", id)
}

// resolveVM fetches the VM row from the store.
func (e *Executor) resolveVM(ctx context.Context, id string) (db.VM, error) {
	nodes, err := e.store.ListNodes(ctx)
	if err != nil {
		return db.VM{}, err
	}
	for _, node := range nodes {
		vms, err := e.store.ListVMsByNode(ctx, node.ID)
		if err != nil {
			continue
		}
		for _, vm := range vms {
			if vm.ID == id {
				return vm, nil
			}
		}
	}
	return db.VM{}, fmt.Errorf("vm %s not found", id)
}

// Ensure *docker.Client satisfies ContainerLifecycle at compile time.
var _ ContainerLifecycle = (*docker.Client)(nil)

// Ensure *proxmox.Client satisfies VMLifecycle at compile time.
var _ VMLifecycle = (*proxmox.Client)(nil)
