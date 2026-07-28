package domain

import "fmt"

type Error struct {
	Code    string
	Message string
	Hint    string
}

func (e *Error) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s\n  hint: %s", e.Code, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func newError(code, msg, hint string) *Error {
	return &Error{Code: code, Message: msg, Hint: hint}
}

func ErrTemplateNotFound(name string) *Error {
	return newError("TEMPLATE_NOT_FOUND",
		fmt.Sprintf("template %q not found in Proxmox", name),
		"run 'proxmox-k3s template validate' or create the template manually (docs/proxmox-setup.md)")
}

func ErrTemplateNotTemplate(name string) *Error {
	return newError("NOT_A_TEMPLATE",
		fmt.Sprintf("%q exists but is not marked as a template (template=1)", name),
		"convert the VM to template: right-click → Convert to Template in the Proxmox UI")
}

func ErrStorageNotFound(name string) *Error {
	return newError("STORAGE_NOT_FOUND",
		fmt.Sprintf("storage %q not found in Proxmox", name),
		"check available storages at Datacenter → Storage in the Proxmox UI")
}

func ErrStorageNoImages(name string) *Error {
	return newError("STORAGE_NO_IMAGES",
		fmt.Sprintf("storage %q does not support 'images' content type", name),
		"enable 'Disk image' content type for this storage in Proxmox")
}

func ErrSnippetStorageNotFound(name string) *Error {
	return newError("SNIPPET_STORAGE_NOT_FOUND",
		fmt.Sprintf("snippets storage %q not found", name),
		"enable 'Snippets' content type on a storage, or remove snippets_storage from config")
}

func ErrNodeNotFound(name string) *Error {
	return newError("PVE_NODE_NOT_FOUND",
		fmt.Sprintf("Proxmox node %q not found or offline", name),
		"check that the node is online at Datacenter → Nodes")
}

func ErrVMIDInUse(vmid int) *Error {
	return newError("VMID_IN_USE",
		fmt.Sprintf("VMID %d is already in use", vmid),
		"the VMID will be retried automatically; if persistent, check vmid_range in config")
}

func ErrInsufficientResources(node, resource string, have, need int) *Error {
	return newError("INSUFFICIENT_RESOURCES",
		fmt.Sprintf("Proxmox node %q has insufficient %s: have %d, need %d", node, resource, have, need),
		"add more PVE nodes to target_nodes, reduce pool sizes, or free resources")
}

func ErrPermissionDenied(endpoint string) *Error {
	return newError("PERMISSION_DENIED",
		fmt.Sprintf("access denied to %q", endpoint),
		"check that the API token role has the required privileges (docs/proxmox-setup.md)")
}

func ErrSizeBelowMinimum(sizeName, role string, field string, got, min int) *Error {
	return newError("SIZE_BELOW_MINIMUM",
		fmt.Sprintf("size %q for %s: %s=%d is below minimum %d", sizeName, role, field, got, min),
		"choose a larger size or increase the resource value; run 'proxmox-k3s sizes' to see the catalog")
}

func ErrIPPoolExhausted(pool string) *Error {
	return newError("IP_POOL_EXHAUSTED",
		fmt.Sprintf("IP pool %q has no free addresses for the requested node count", pool),
		"expand node_pool_range in the networking section")
}

func ErrIPConflict(ip string) *Error {
	return newError("IP_CONFLICT",
		fmt.Sprintf("IP %q is already in use on the network", ip),
		"remove the conflicting host or choose a different IP range")
}

func ErrVIPConflict(vip string) *Error {
	return newError("VIP_CONFLICT",
		fmt.Sprintf("control_plane_vip %q is already responding on the network", vip),
		"choose a free IP for the VIP, outside of node_pool_range and metallb address_pool")
}

func ErrOddMasterCount(count int) *Error {
	return newError("ODD_MASTER_COUNT",
		fmt.Sprintf("masters_pool.count must be odd and >= 3, got %d", count),
		"use count: 3, 5, or 7 to ensure etcd quorum")
}

func ErrClusterNotFound(name string) *Error {
	return newError("CLUSTER_NOT_FOUND",
		fmt.Sprintf("no VMs tagged with cluster %q found in Proxmox", name),
		"check cluster_name in your config matches the deployed cluster")
}

func ErrSSHConnect(host string, err error) *Error {
	return newError("SSH_CONNECT_FAILED",
		fmt.Sprintf("cannot connect to %s via SSH: %v", host, err),
		"verify the node has booted, SSH is running, and the key in ssh.private_key_path is correct")
}

func ErrK3sInstall(node string, err error) *Error {
	return newError("K3S_INSTALL_FAILED",
		fmt.Sprintf("K3s installation failed on %s: %v", node, err),
		"check the install log printed above; common causes: network connectivity, wrong K3s version, SELinux")
}
