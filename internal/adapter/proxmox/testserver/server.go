// Package testserver provides a fake Proxmox VE HTTP API for use in tests.
// It tracks state (VMs, tasks) and supports injecting errors per operation.
package testserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FakeVM represents a VM tracked by the fake server.
type FakeVM struct {
	VMID     int
	Name     string
	Node     string
	Tags     string
	Template int
	Status   string
	CPU      int
	Memory   int
}

// FakeNode represents a PVE host node.
type FakeNode struct {
	Name   string
	Status string
	MaxCPU int
	MaxMem int64
}

// ErrorInjector lets tests force specific operations to fail.
type ErrorInjector struct {
	mu      sync.Mutex
	entries map[string]error
}

func (e *ErrorInjector) Set(op string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.entries == nil {
		e.entries = make(map[string]error)
	}
	e.entries[op] = err
}

func (e *ErrorInjector) Clear(op string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.entries, op)
}

func (e *ErrorInjector) get(op string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.entries[op]
}

// Server is a fake Proxmox VE API server for integration testing.
type Server struct {
	mu       sync.Mutex
	vms      map[int]*FakeVM
	nodes    []FakeNode
	nextVMID int32
	upidSeq  int32
	tasks    map[string]bool // upid → completed

	Errors *ErrorInjector
	// Recorded operations for assertions
	Ops []string

	httpServer *httptest.Server
}

// New creates and starts a fake Proxmox server with one online PVE node.
func New() *Server {
	s := &Server{
		vms:      make(map[int]*FakeVM),
		tasks:    make(map[string]bool),
		nextVMID: 100,
		Errors:   &ErrorInjector{},
		nodes: []FakeNode{
			{Name: "pve1", Status: "online", MaxCPU: 32, MaxMem: 64 * 1024 * 1024 * 1024},
		},
	}

	// Pre-add a template VM
	s.vms[9000] = &FakeVM{
		VMID:     9000,
		Name:     "ubuntu-2404-cloudinit",
		Node:     "pve1",
		Template: 1,
		Status:   "stopped",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", s.handleVersion)
	mux.HandleFunc("/api2/json/nodes", s.handleNodes)
	mux.HandleFunc("/api2/json/cluster/resources", s.handleClusterResources)
	mux.HandleFunc("/api2/json/cluster/nextid", s.handleNextID)
	mux.HandleFunc("/", s.handleDynamic)

	s.httpServer = httptest.NewServer(mux)
	return s
}

// URL returns the base URL of the fake server.
func (s *Server) URL() string { return s.httpServer.URL }

// Close shuts down the fake server.
func (s *Server) Close() { s.httpServer.Close() }

// VMCount returns the number of non-template VMs currently tracked.
func (s *Server) VMCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, vm := range s.vms {
		if vm.Template == 0 {
			n++
		}
	}
	return n
}

// VMs returns a copy of all non-template VMs.
func (s *Server) VMs() []*FakeVM {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*FakeVM, 0, len(s.vms))
	for _, vm := range s.vms {
		if vm.Template == 0 {
			cp := *vm
			out = append(out, &cp)
		}
	}
	return out
}

// AddNode adds an additional PVE host node.
func (s *Server) AddNode(n FakeNode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = append(s.nodes, n)
}

func (s *Server) respond(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func (s *Server) respondError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"errors": msg})
}

func (s *Server) newUPID(node, op string) string {
	seq := atomic.AddInt32(&s.upidSeq, 1)
	upid := fmt.Sprintf("UPID:%s:00%04X:00000001:%X:%s::root@pam:", node, seq, time.Now().Unix(), op)
	s.mu.Lock()
	s.tasks[upid] = true
	s.mu.Unlock()
	return upid
}

func (s *Server) recordOp(op string) {
	s.mu.Lock()
	s.Ops = append(s.Ops, op)
	s.mu.Unlock()
}

// -- Handlers --

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.respond(w, map[string]any{"version": "8.2.0", "release": "8"})
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	nodes := make([]map[string]any, 0, len(s.nodes))
	for _, n := range s.nodes {
		nodes = append(nodes, map[string]any{
			"node":    n.Name,
			"status":  n.Status,
			"maxcpu":  n.MaxCPU,
			"maxmem":  n.MaxMem,
			"cpu":     0.1,
			"mem":     1 * 1024 * 1024 * 1024,
		})
	}
	s.mu.Unlock()
	s.respond(w, nodes)
}

func (s *Server) handleClusterResources(w http.ResponseWriter, r *http.Request) {
	resType := r.URL.Query().Get("type")
	s.mu.Lock()
	defer s.mu.Unlock()

	var resources []map[string]any

	if resType == "node" || resType == "" {
		for _, n := range s.nodes {
			resources = append(resources, map[string]any{
				"type":   "node",
				"node":   n.Name,
				"status": n.Status,
				"maxcpu": n.MaxCPU,
				"maxmem": n.MaxMem,
			})
		}
	}

	if resType == "vm" || resType == "" {
		for _, vm := range s.vms {
			resources = append(resources, map[string]any{
				"type":     "qemu",
				"vmid":     vm.VMID,
				"name":     vm.Name,
				"node":     vm.Node,
				"status":   vm.Status,
				"tags":     vm.Tags,
				"template": vm.Template,
			})
		}
	}

	s.respond(w, resources)
}

func (s *Server) handleNextID(w http.ResponseWriter, r *http.Request) {
	id := int(atomic.AddInt32(&s.nextVMID, 1))
	s.respond(w, id)
}

// handleDynamic dispatches all path-parameterized routes.
var (
	reClone  = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/qemu/(\d+)/clone$`)
	reConfig = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/qemu/(\d+)/config$`)
	reResize = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/qemu/(\d+)/resize$`)
	reStart  = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/qemu/(\d+)/status/start$`)
	reStop   = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/qemu/(\d+)/status/stop$`)
	reDelete = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/qemu/(\d+)`)
	reTask   = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/tasks/([^/]+)/status$`)
	reAgent  = regexp.MustCompile(`^/api2/json/nodes/([^/]+)/qemu/(\d+)/agent/network-get-interfaces$`)
)

func (s *Server) handleDynamic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if m := reTask.FindStringSubmatch(path); m != nil {
		s.handleTaskStatus(w, r, m[1], m[2])
		return
	}
	if m := reAgent.FindStringSubmatch(path); m != nil {
		s.handleAgent(w, r, m[1], mustInt(m[2]))
		return
	}
	if m := reClone.FindStringSubmatch(path); m != nil && r.Method == http.MethodPost {
		s.handleClone(w, r, m[1], mustInt(m[2]))
		return
	}
	if m := reConfig.FindStringSubmatch(path); m != nil && r.Method == http.MethodPost {
		s.handleConfig(w, r, m[1], mustInt(m[2]))
		return
	}
	if m := reResize.FindStringSubmatch(path); m != nil && r.Method == http.MethodPut {
		s.handleResize(w, r, m[1], mustInt(m[2]))
		return
	}
	if m := reStart.FindStringSubmatch(path); m != nil && r.Method == http.MethodPost {
		s.handleStart(w, r, m[1], mustInt(m[2]))
		return
	}
	if m := reStop.FindStringSubmatch(path); m != nil && r.Method == http.MethodPost {
		s.handleStop(w, r, m[1], mustInt(m[2]))
		return
	}
	if m := reDelete.FindStringSubmatch(path); m != nil && r.Method == http.MethodDelete {
		s.handleDelete(w, r, m[1], mustInt(m[2]))
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleTaskStatus(w http.ResponseWriter, r *http.Request, node, upid string) {
	s.mu.Lock()
	_, exists := s.tasks[upid]
	s.mu.Unlock()

	if !exists {
		s.respondError(w, 404, "task not found")
		return
	}
	s.respond(w, map[string]any{
		"status":     "stopped",
		"exitstatus": "OK",
		"type":       "qmclone",
		"id":         upid,
	})
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request, node string, vmid int) {
	if err := s.Errors.get("agent"); err != nil {
		s.respondError(w, 500, err.Error())
		return
	}

	s.mu.Lock()
	vm, ok := s.vms[vmid]
	s.mu.Unlock()

	if !ok {
		s.respondError(w, 404, "vm not found")
		return
	}

	ip := vmIP(vm.Name)
	s.respond(w, map[string]any{
		"result": []map[string]any{
			{
				"name":             "eth0",
				"hardware-address": "52:54:00:12:34:56",
				"ip-addresses": []map[string]any{
					{"ip-address": ip, "ip-address-type": "ipv4", "prefix": 24},
				},
			},
		},
	})
}

func (s *Server) handleClone(w http.ResponseWriter, r *http.Request, node string, templateVMID int) {
	s.recordOp("clone")
	if err := s.Errors.get("clone"); err != nil {
		s.respondError(w, 500, err.Error())
		return
	}

	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)

	newID := int(body["newid"].(float64))
	name, _ := body["name"].(string)

	s.mu.Lock()
	s.vms[newID] = &FakeVM{
		VMID:   newID,
		Name:   name,
		Node:   node,
		Status: "stopped",
	}
	s.mu.Unlock()

	s.respond(w, s.newUPID(node, "qmclone"))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request, node string, vmid int) {
	s.recordOp("config")
	if err := s.Errors.get("config"); err != nil {
		s.respondError(w, 500, err.Error())
		return
	}

	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)

	s.mu.Lock()
	if vm, ok := s.vms[vmid]; ok {
		if tags, ok := body["tags"].(string); ok {
			vm.Tags = tags
		}
		if cpu, ok := body["cores"].(float64); ok {
			vm.CPU = int(cpu)
		}
		if mem, ok := body["memory"].(float64); ok {
			vm.Memory = int(mem)
		}
	}
	s.mu.Unlock()

	// config can return empty string (no task) or a UPID
	s.respond(w, "")
}

func (s *Server) handleResize(w http.ResponseWriter, r *http.Request, node string, vmid int) {
	s.recordOp("resize")
	s.respond(w, s.newUPID(node, "qmresize"))
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request, node string, vmid int) {
	s.recordOp("start")
	if err := s.Errors.get("start"); err != nil {
		s.respondError(w, 500, err.Error())
		return
	}

	s.mu.Lock()
	if vm, ok := s.vms[vmid]; ok {
		vm.Status = "running"
	}
	s.mu.Unlock()

	s.respond(w, s.newUPID(node, "qmstart"))
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request, node string, vmid int) {
	s.recordOp("stop")
	s.mu.Lock()
	if vm, ok := s.vms[vmid]; ok {
		vm.Status = "stopped"
	}
	s.mu.Unlock()
	s.respond(w, s.newUPID(node, "qmstop"))
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, node string, vmid int) {
	s.recordOp("delete")
	if err := s.Errors.get("delete"); err != nil {
		s.respondError(w, 500, err.Error())
		return
	}

	s.mu.Lock()
	delete(s.vms, vmid)
	s.mu.Unlock()

	s.respond(w, s.newUPID(node, "qmdestroy"))
}

// vmIP returns a deterministic fake IP based on the VM name.
func vmIP(name string) string {
	parts := strings.Split(name, "-")
	idx := 0
	if len(parts) > 0 {
		for _, ch := range parts[len(parts)-1] {
			idx += int(ch)
		}
	}
	return fmt.Sprintf("192.168.20.%d", 50+idx%50)
}

func mustInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
