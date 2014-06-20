package fbp

import (
	"strings"
)

//
// Process of FBP flow
//
type Process struct {
	Name      string            `json:"-"`
	Component string            `json:"component"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func (p *Process) String() string {
	return p.Name + "(" + p.Component + ")"
}

//
// Endpoint (in/out port of a Process)
//
type Endpoint struct {
	Process string `json:"process"`
	Port    string `json:"port"`
}

func (e *Endpoint) String() string {
	return "(" + e.Process + ", " + e.Port + ")"
}

//
// Connection (arc) between endpoints
//
type Connection struct {
	Data   string    `json:"data,omitempty"`
	Source *Endpoint `json:"src,omitempty"`
	Target *Endpoint `json:"tgt"`
}

func (c *Connection) String() string {
	if c.Data != "" {
		return "(" + c.Data + " -> " + c.Target.String() + ")"
	} else if c.Source != nil {
		return "(" + c.Source.String() + " -> " + c.Target.String() + ")"
	} else {
		return "(????????? -> " + c.Target.String() + ")"
	}
}

//
// In/Out ports for composites
//

//
// Base structure for FBP parser to inherit
//
type BaseFbp struct {
	// Private variables to keep state during .fbp parsing
	iip               string
	port              string
	inPort            string
	outPort           string
	nodeProcessName   string
	nodeComponentName string
	nodeMeta          string
	srcEndpoint       *Endpoint
	tgtEndpoint       *Endpoint

	// Reference to a name of the composite (if any)
	Subgraph string

	// Keeps parsed processes
	Processes []*Process
	// Keeps parsed connections
	Connections []*Connection

	// In/Out ports to export outside (composite components)
	Inports  map[string]*Endpoint
	Outports map[string]*Endpoint
}

func (self *BaseFbp) createProcessName(name string) string {
	if self.Subgraph != "" {
		return self.Subgraph + "_" + name
	}
	return name
}

func (self *BaseFbp) createLeftlet() {
	//log.Println("createLeftlet()", self.nodeProcessName, self.port)
	self.srcEndpoint = &Endpoint{
		Process: self.createProcessName(self.nodeProcessName),
		Port:    self.port,
	}
	self.nodeProcessName = ""
	self.port = ""
}

func (self *BaseFbp) createRightlet() {
	//log.Println("createRightlet()", self.nodeProcessName, self.port)
	self.tgtEndpoint = &Endpoint{
		Process: self.createProcessName(self.nodeProcessName),
		Port:    self.port,
	}
	var connection *Connection
	if self.srcEndpoint != nil {
		connection = &Connection{
			Source: self.srcEndpoint,
			Target: self.tgtEndpoint,
		}
	} else {
		connection = &Connection{
			Data:   self.iip,
			Target: self.tgtEndpoint,
		}
	}
	self.Connections = append(self.Connections, connection)

	self.nodeProcessName = ""
	self.port = ""
	self.srcEndpoint = nil
	self.tgtEndpoint = nil
	self.iip = ""
}

func (self *BaseFbp) createMiddlet() {
	//log.Println("createMiddlet()")
	self.tgtEndpoint = &Endpoint{
		Process: self.createProcessName(self.nodeProcessName),
		Port:    self.inPort,
	}

	var connection *Connection
	if self.srcEndpoint != nil {
		connection = &Connection{
			Source: self.srcEndpoint,
			Target: self.tgtEndpoint,
		}
	} else {
		connection = &Connection{
			Data:   self.iip,
			Target: self.tgtEndpoint,
		}
	}
	self.Connections = append(self.Connections, connection)

	self.port = self.outPort
	self.inPort = ""
	self.outPort = ""
	self.createLeftlet()
}

func (self *BaseFbp) createNode() {
	if self.nodeComponentName != "" && !self.processExists(self.nodeProcessName) {
		process := &Process{
			Name:      self.createProcessName(self.nodeProcessName),
			Component: self.nodeComponentName,
		}
		if self.nodeMeta != "" {
			m := make(map[string]string)
			pairs := strings.Split(self.nodeMeta, ",")
			for _, v := range pairs {
				kv := strings.SplitN(v, "=", 2)
				if len(kv) < 2 {
					m[strings.TrimSpace(kv[0])] = ""
				} else {
					m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
			process.Metadata = m
		}
		self.nodeMeta = ""
		self.Processes = append(self.Processes, process)
	}
}

func (self *BaseFbp) processExists(name string) bool {
	for _, ps := range self.Processes {
		if ps.Name == self.createProcessName(name) {
			return true
		}
	}
	return false
}

func (self *BaseFbp) parseExportedPort(str string) (name string, endpoint *Endpoint) {
	// str = component.port:externalport
	parts := strings.Split(str, ":")
	if len(parts) != 2 {
		return "", nil
	}
	name = strings.TrimSpace(parts[1])
	parts = strings.Split(parts[0], ".")
	endpoint = &Endpoint{}
	endpoint.Port = strings.TrimSpace(parts[1])
	endpoint.Process = self.createProcessName(strings.TrimSpace(parts[0]))
	return name, endpoint
}

func (self *BaseFbp) createInport(str string) {
	port, endpoint := self.parseExportedPort(str)
	if endpoint == nil {
		return
	}
	if self.Inports == nil {
		self.Inports = make(map[string]*Endpoint)
	}
	self.Inports[port] = endpoint
}

func (self *BaseFbp) createOutport(str string) {
	port, endpoint := self.parseExportedPort(str)
	if endpoint == nil {
		return
	}
	if self.Outports == nil {
		self.Outports = make(map[string]*Endpoint)
	}
	self.Outports[port] = endpoint
}

func (self *BaseFbp) Validate() error {
	//TODO: check if the network can be executed (it can conform to PEG but be invalid)
	// - Process without component (compare # of components with # of processes)
	// - Check if all endpoints in connections are in the processes
	// - etc
	return nil
}
