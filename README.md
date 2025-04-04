# Virtual BMC for VMware vSphere

This project implements a virtual Baseboard Management Controller (BMC) for VMware virtual machines. It creates IPMI endpoints for each VM in a specified datacenter or folder, allowing standard IPMI tools to interact with the VMs for power management operations.

## Features

- Creates virtual BMC endpoints for VMware VMs
- Supports basic IPMI operations (power on/off, status)
- Configurable port range for IPMI servers
- Optional folder-based VM filtering

## Building

```bash
go build -o vbmc-vsphere
```

## Configuration

Create a JSON configuration file (e.g., `config.json`) with the following structure:

```json
{
    "vcenter": {
        "ip": "vcenter.example.com",
        "user": "administrator@vsphere.local",
        "password": "your-password",
        "datacenter": "your-datacenter",
        "folder": "optional/folder/path"
    },
    "server": {
        "listen_ip": "0.0.0.0",
        "start_port": 6230
    }
}
```

### Configuration Fields

#### vCenter Section
- `ip`: vCenter server IP address or hostname (required)
- `user`: vCenter username (required)
- `password`: vCenter password (required)
- `datacenter`: vCenter datacenter name (required)
- `folder`: vCenter folder path to filter VMs (optional)

#### Server Section
- `ip_range`: Configuration for the IP address range
  - `start`: First IP address in the range (required)
  - `end`: Last IP address in the range (required)

The virtual BMC will assign one IP address from the range to each VM. Each BMC will listen on the standard IPMI port (623).

An example configuration file is provided as `config.json.example`.

## Usage

```bash
./vbmc-vsphere [-config path/to/config.json]
```

### Arguments

- `-config`: Path to configuration file (default: "config.json")

## IPMI Client Usage

Once the virtual BMC is running, you can use standard IPMI tools to interact with the VMs. Each VM will be assigned a unique IP address from the configured range.

Example using ipmitool:

```bash
# Get power status
ipmitool -I lanplus -H <vm-ip> -p 623 -U admin -P password power status

# Power on VM
ipmitool -I lanplus -H <vm-ip> -p 623 -U admin -P password power on

# Power off VM
ipmitool -I lanplus -H <vm-ip> -p 623 -U admin -P password power off

# Set boot device to CD/DVD
ipmitool -I lanplus -H <vm-ip> -p 623 -U admin -P password chassis bootdev cdrom

# Set boot device to PXE
ipmitool -I lanplus -H <vm-ip> -p 623 -U admin -P password chassis bootdev pxe

# Set boot device to HDD
ipmitool -I lanplus -H <vm-ip> -p 623 -U admin -P password chassis bootdev disk

# Set boot device to floppy
ipmitool -I lanplus -H <vm-ip> -p 623 -U admin -P password chassis bootdev floppy
```

### Supported Boot Devices

The virtual BMC supports the following boot devices:
- HDD (disk)
- CD/DVD (cdrom)
- PXE (network)
- Floppy

When you set a boot device, it will be used for the next boot only. The VM will revert to its default boot order after the next reboot.
