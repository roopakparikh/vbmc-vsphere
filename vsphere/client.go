package vsphere

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Client represents a vSphere client
type Client struct {
	client     *govmomi.Client
	finder     *find.Finder
	datacenter *object.Datacenter
}

// NewClient creates a new vSphere client
func NewClient(ctx context.Context, vcenterIP, username, password, datacenter string) (*Client, error) {
	u, err := url.Parse(fmt.Sprintf("https://%s/sdk", vcenterIP))
	if err != nil {
		return nil, fmt.Errorf("failed to parse vCenter URL: %v", err)
	}
	u.User = url.UserPassword(username, password)

	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create vSphere client: %v", err)
	}

	finder := find.NewFinder(client.Client, true)
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to find datacenter: %v", err)
	}
	finder.SetDatacenter(dc)

	return &Client{
		client:     client,
		finder:     finder,
		datacenter: dc,
	}, nil
}

// GetVMs returns all VMs in the specified folder or datacenter
func (c *Client) GetVMs(ctx context.Context, folderPath string) ([]*object.VirtualMachine, error) {
	var vms []*object.VirtualMachine
	var err error

	if folderPath != "" {
		folder, err := c.finder.Folder(ctx, folderPath)
		if err != nil {
			return nil, fmt.Errorf("failed to find folder: %v", err)
		}
		vms, err = c.finder.VirtualMachineList(ctx, folder.InventoryPath+"/*")
	} else {
		vms, err = c.finder.VirtualMachineList(ctx, "*")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %v", err)
	}

	return vms, nil
}

// GetVMPowerState returns the power state of a VM
func (c *Client) GetVMPowerState(ctx context.Context, vm *object.VirtualMachine) (string, error) {
	var o mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"runtime.powerState"}, &o)
	if err != nil {
		return "", fmt.Errorf("failed to get VM properties: %v", err)
	}
	return string(o.Runtime.PowerState), nil
}

// PowerOnVM powers on a VM
func (c *Client) PowerOnVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("failed to power on VM: %v", err)
	}
	return task.Wait(ctx)
}

// PowerOffVM powers off a VM
func (c *Client) PowerOffVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("failed to power off VM: %v", err)
	}
	return task.Wait(ctx)
}

// BootDevice represents a VM boot device
type BootDevice string

const (
	BootDeviceHDD    BootDevice = "hdd"
	BootDeviceCDROM  BootDevice = "cdrom"
	BootDevicePXE    BootDevice = "pxe"
	BootDeviceFloppy BootDevice = "floppy"
)

// SetNextBoot sets the next boot device for a VM
func (c *Client) SetNextBoot(ctx context.Context, vm *object.VirtualMachine, device BootDevice) error {
	var err error
	var bootOptions *types.VirtualMachineBootOptions

	// Get current configuration
	var vmConfig mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config"}, &vmConfig)
	if err != nil {
		return fmt.Errorf("failed to get VM config: %v", err)
	}

	// Create boot options if they don't exist
	if vmConfig.Config.BootOptions == nil {
		bootOptions = &types.VirtualMachineBootOptions{}
	} else {
		bootOptions = vmConfig.Config.BootOptions
	}

	// Set boot order based on device
	switch device {
	case BootDeviceHDD:
		bootOptions.BootOrder = []types.BaseVirtualMachineBootOptionsBootableDevice{
			&types.VirtualMachineBootOptionsBootableDiskDevice{},
		}
	case BootDeviceCDROM:
		bootOptions.BootOrder = []types.BaseVirtualMachineBootOptionsBootableDevice{
			&types.VirtualMachineBootOptionsBootableCdromDevice{},
		}
	case BootDevicePXE:
		bootOptions.BootOrder = []types.BaseVirtualMachineBootOptionsBootableDevice{
			&types.VirtualMachineBootOptionsBootableEthernetDevice{},
		}
	case BootDeviceFloppy:
		bootOptions.BootOrder = []types.BaseVirtualMachineBootOptionsBootableDevice{
			&types.VirtualMachineBootOptionsBootableFloppyDevice{},
		}
	default:
		return fmt.Errorf("unsupported boot device: %s", device)
	}

	// Create spec for reconfiguration
	spec := types.VirtualMachineConfigSpec{
		BootOptions: bootOptions,
	}

	// Apply the configuration
	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("failed to reconfigure VM: %v", err)
	}

	return task.Wait(ctx)
}
