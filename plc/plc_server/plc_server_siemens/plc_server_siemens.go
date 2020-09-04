package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"

	"errors"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/davecourtois/Globular/Interceptors"
	"github.com/davecourtois/Globular/plc/plc_client"
	"github.com/davecourtois/Globular/plc/plcpb"
	"github.com/davecourtois/Utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"

	//	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"fmt"
	"math"
	"time"

	"github.com/robinson/gos7"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10025
	defaultProxy = 10026

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	domain string = "localhost"
)

type TagType int

const (
	BOOL_TAG_TYPE   TagType = 0
	SINT_TAG_TYPE   TagType = 1
	INT_TAG_TYPE    TagType = 2
	DINT_TAG_TYPE   TagType = 3
	REAL_TAG_TYPE   TagType = 4
	LREAL_TAG_TYPE  TagType = 5
	LINT_TAG_TYPE   TagType = 6
	UNKNOW_TAG_TYPE TagType = 7
)

type PortType int

const (
	SERIAL PortType = 4
	TCP    PortType = 5
)

// Keep connection information here.
type connection struct {
	Id      string // The connection id
	IP      string // can also be ipv4 addresse.
	Port    PortType
	Slot    int32
	Rack    int32
	Timeout int64 // Time out for reading/writing tags

	handler *gos7.TCPClientHandler
	client  gos7.Client
	isOpen  bool
}

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Domain          string
	PublisherId     string

	// self-signed X.509 public keys for distribution
	CertFile string
	// a private RSA key to sign and authenticate the public key
	KeyFile string
	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string
	TLS                bool
	Version            string
	KeepUpToDate       bool
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.

	// use only for serialization.
	Connections []connection

	// The map of connection...
	connections map[string]*connection
}

// Create the configuration file if is not already exist.
func (self *server) init() {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewPlc_Client", plc_client.NewPlc_Client)

	// Here I will retreive the list of connections from file if there are some...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	file, err := ioutil.ReadFile(dir + "/config.json")
	if err == nil {
		err := json.Unmarshal([]byte(file), &self)
		if err == nil {
			for i := 0; i < len(self.Connections); i++ {
				c := &self.Connections[i]
				// Create the connection with the plc.
				fmt.Println("connect with plc at adress ", c.IP, " at slot ", c.Slot, " and rack ", c.Rack)
				c.handler = gos7.NewTCPClientHandler(c.IP, int(c.Rack), int(c.Slot))
				c.client = gos7.NewClient(c.handler)

				if err != nil {
					fmt.Println(err)
					fmt.Println("Fail to connect to ", c.IP, err)
				} else {

					self.connections[c.Id] = c
				}
			}
		} else {
			fmt.Println(err)
		}
	} else {
		// save it the first time to generate the configuratio file.
		if len(self.Id) == 0 {
			// Generate random id for the server instance.
			self.Id = Utility.RandomUUID()
		}
		self.save()

	}
}

// Save the configuration values.
func (self *server) save() error {

	// Create the file...
	str, err := Utility.ToJson(self)
	if err != nil {
		return err
	}

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}

	ioutil.WriteFile(dir+"/config.json", []byte(str), 0644)
	return nil
}

////////////////// Now the API ////////////////

// Create a new connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (self *server) CreateConnection(ctx context.Context, rqst *plcpb.CreateConnectionRqst) (*plcpb.CreateConnectionRsp, error) {

	fmt.Println("Try to create a new connection")
	var c connection

	// Set the connection info from the request.
	c.Id = rqst.Connection.Id
	c.IP = rqst.Connection.Ip
	c.Rack = rqst.Connection.Rack
	c.Port = PortType(rqst.Connection.PortType)
	c.Slot = rqst.Connection.Slot
	c.Timeout = rqst.Connection.Timeout

	// Create the connection with the plc.
	c.handler = gos7.NewTCPClientHandler(c.IP, int(c.Rack), int(c.Slot))
	c.client = gos7.NewClient(c.handler)

	if rqst.Save == true {
		if self.connections[c.Id] == nil {
			self.Connections = append(self.Connections, c)
		} else {
			// Here I will put all connections in the Connections array.
			self.Connections = make([]connection, 0)
			for _, c_ := range self.connections {
				if c_.Id != c.Id {
					self.Connections = append(self.Connections, *c_)
				} else {
					self.Connections = append(self.Connections, c)
				}
			}
		}
	}

	// set the connection.
	self.connections[c.Id] = &c

	// Print the success message here.
	fmt.Println("Connection " + c.Id + " was created with success!")

	return &plcpb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Retreive a connection from the map of connection.
func (self *server) GetConnection(ctx context.Context, rqst *plcpb.GetConnectionRqst) (*plcpb.GetConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := self.connections[id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection with id "+id+" does not exist!")))
	}

	c := self.connections[id]
	// return success.
	return &plcpb.GetConnectionRsp{
		Connection: &plcpb.Connection{Id: c.Id, Ip: c.IP, Rack: c.Rack, Slot: c.Slot, Timeout: c.Timeout, PortType: plcpb.PortType(c.Port), Cpu: plcpb.CpuType_SIMMENS},
	}, nil
}

// Remove a connection from the map and the file.
func (self *server) DeleteConnection(ctx context.Context, rqst *plcpb.DeleteConnectionRqst) (*plcpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := self.connections[id]; !ok {
		return &plcpb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	// close the plc connection if it;s open
	self.connections[id].handler.Close()

	self.Connections = make([]connection, 0)
	c := self.connections[id]
	for _, c_ := range self.connections {
		if c_.Id != c.Id {
			self.Connections = append(self.Connections, *c)
		}
	}

	// also remove it from the map
	delete(self.connections, id)

	// In that case I will save it in file.
	err := self.save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &plcpb.DeleteConnectionRsp{
		Result: true,
	}, nil
}

// Close a connection
func (self *server) CloseConnection(ctx context.Context, rqst *plcpb.CloseConnectionRqst) (*plcpb.CloseConnectionRsp, error) {
	for _, c := range self.connections {

		err := c.handler.Close()
		c.isOpen = false

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	return &plcpb.CloseConnectionRsp{
		Result: true,
	}, nil
}

// Return the size of a tag.
func (self *server) getTagTypeSize(tagType TagType) int {
	if tagType == BOOL_TAG_TYPE || tagType == SINT_TAG_TYPE {
		return 1
	} else if tagType == INT_TAG_TYPE {
		return 2
	} else if tagType == DINT_TAG_TYPE || tagType == REAL_TAG_TYPE {
		return 4
	} else if tagType == LINT_TAG_TYPE || tagType == LREAL_TAG_TYPE {
		return 8
	}

	return -1 // must not be taken.
}

// Read Tag
func (self *server) ReadTag(ctx context.Context, rqst *plcpb.ReadTagRqst) (*plcpb.ReadTagRsp, error) {

	// first of all I will retreive the connection.
	if c, ok := self.connections[rqst.ConnectionId]; ok {
		if c.isOpen == false {
			c.handler.Connect()
			c.isOpen = true
		}

		tagType := TagType(int(rqst.Type))
		size := self.getTagTypeSize(tagType)
		if size == -1 {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No datatype size was found for tag "+rqst.Name)))
		}

		length := int(rqst.Length)
		offset := int(rqst.Offset)
		name := strings.ToUpper(rqst.Name)
		number := Utility.ToInt(name[2:])

		var err error
		buf := make([]byte, size*length)

		values := make([]interface{}, 0)

		// Read the value from the buffer and return the response.
		var s7 gos7.Helper

		if strings.HasPrefix(name, "DB") {
			//Read data blocks from PLC
			if tagType == BOOL_TAG_TYPE {
				// Here I read bits and not bytes, so I need to divide value by
				// 8 bits.
				size := int(math.Ceil(float64(length) / 8.0))
				offset := int(offset / 8)
				err = c.client.AGReadDB(number, offset, size+offset, buf)
			} else {
				err = c.client.AGReadDB(number, offset*size, size*length, buf)
			}
		} else if strings.HasPrefix(name, "MB") {
			//Read Merkers area from PLC
			err = c.client.AGReadMB(number, size*length, buf)

		} else if strings.HasPrefix(name, "EB") {
			//Read IPI from PLC
			err = c.client.AGReadEB(number, size*length, buf)

		} else if strings.HasPrefix(name, "AB") {
			//Read IPU from PLC
			err = c.client.AGReadAB(number, size*length, buf)

		} else if strings.HasPrefix(name, "T") {
			//Read timer from PLC
			err = c.client.AGReadAB(number, size*length, buf)

		} else if strings.HasPrefix(name, "C") {
			//Read counter from PLC
			err = c.client.AGReadCT(number, size*length, buf)

		} else {
			err = errors.New("No data type found for " + name)
		}

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if strings.HasPrefix(name, "DB") || strings.HasPrefix(name, "MB") || strings.HasPrefix(name, "EB") || strings.HasPrefix(name, "AB") {
			if tagType == BOOL_TAG_TYPE {
				size := int(math.Ceil(float64(length) / 8.0))
				var j = offset % 8 // start at ask offset by skipping complete group of 8 bit's
				for i := 0; i < size; i++ {
					for ; j < 8 && len(values) < length; j++ {
						b := s7.GetBoolAt(buf[i], uint(j))
						if b {
							values = append(values, uint8(1))
						} else {
							values = append(values, uint8(0))
						}
					}
					j = 0
				}

			} else {

				for i := 0; i < length*size; i = i + size {
					if tagType == SINT_TAG_TYPE {
						if rqst.GetUnsigned() {
							values = append(values, uint8(buf[i]))
						} else {
							values = append(values, int8(buf[i]))
						}
					} else if tagType == INT_TAG_TYPE {
						if rqst.GetUnsigned() {
							var result uint16
							s7.GetValueAt(buf, i, &result)
							values = append(values, result)
						} else {
							var result int16
							s7.GetValueAt(buf, i, &result)
							values = append(values, result)
						}

					} else if tagType == DINT_TAG_TYPE {
						if rqst.GetUnsigned() {
							var result uint32
							s7.GetValueAt(buf, i, &result)
							values = append(values, result)
						} else {
							var result int32
							s7.GetValueAt(buf, i, &result)
							values = append(values, result)
						}
					} else if tagType == LINT_TAG_TYPE {
						if rqst.GetUnsigned() {
							var result uint64
							s7.GetValueAt(buf, i, &result)
							values = append(values, result)
						} else {
							var result int64
							s7.GetValueAt(buf, i, &result)
							values = append(values, result)
						}
					} else if tagType == REAL_TAG_TYPE {
						values = append(values, s7.GetRealAt(buf, i))
					} else if tagType == LREAL_TAG_TYPE {
						values = append(values, s7.GetLRealAt(buf, i))
					} else {
						return nil, status.Errorf(
							codes.Internal,
							Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Tag type not found!")))
					}
				}
			}
		} else if strings.HasPrefix(name, "C") {
			fmt.Println("---> CT not implemented!")
			//Read counter from PLC
			/*for i := 0; i < length*size; i = i + size {
				s7.GetCounterAt(buf, i)
			}*/

		} else if strings.HasPrefix(name, "T") {
			for i := 0; i < length*size; i = i + size {
				t := s7.GetDateTimeAt(buf, i)
				values = append(values, t.Unix())
			}
		}
		// return the values as string.
		jsonStr, _ := Utility.ToJson(values)
		return &plcpb.ReadTagRsp{
			Values: jsonStr,
		}, nil

	} else {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No connection found with id "+rqst.ConnectionId)))
	}

}

// Write Tag
func (self *server) WriteTag(ctx context.Context, rqst *plcpb.WriteTagRqst) (*plcpb.WriteTagRsp, error) {

	// first of all I will retreive the connection.
	if c, ok := self.connections[rqst.ConnectionId]; ok {

		if c.isOpen == false {
			c.handler.Connect()
			c.isOpen = true
		}
		tagType := TagType(int(rqst.Type))
		size := self.getTagTypeSize(tagType)
		if size == -1 {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No datatype size was found for tag "+rqst.Name)))
		}

		offset := int(rqst.Offset)
		length := int(rqst.Length)
		name := strings.ToUpper(rqst.Name)
		number := Utility.ToInt(name[2:])

		var err error
		var buf []byte
		if tagType == BOOL_TAG_TYPE {
			// The length given in parameter reprente the bit's here and not
			// the byte's so I must divide by 8 (the lengt of Byte's in Bit's)
			size := int(math.Ceil(float64(length) / 8.0))
			if offset%8 != 0 {
				size++
			}
			buf = make([]byte, size)

			offset := int(offset / 8)
			// Init the buffer with actual values.
			err = c.client.AGReadDB(number, offset, size, buf)
		} else {
			buf = make([]byte, size*length) // large enought buffer allocated.
		}
		var helper gos7.Helper

		values := make([]interface{}, 0)
		json.Unmarshal([]byte(rqst.Values), &values)

		// Convert the receive string value and write it to the buffer.
		if tagType == BOOL_TAG_TYPE {
			// each value represent a bit.
			offset_ := offset % 8 // the actual bit offset in the buffer.
			//fmt.Println("511 bool buffer length is ", len(buf), "values length is ", len(values))

			for i := 0; i < len(values); i++ {
				index := offset_ + i     // the bit index in the array of bits...
				index_ := int(index / 8) // The index in the buffer.
				//fmt.Println("516 write bit ", index, " of bytes ", index_, " with value ", values[i])
				if Utility.ToInt(values[i]) == 1 {
					buf[index_] = helper.SetBoolAt(buf[index_], uint(index%8), true)
				} else {
					buf[index_] = helper.SetBoolAt(buf[index_], uint(index%8), false)
				}
			}
		} else {
			index := 0
			for i := 0; i < length*size; i = i + size {
				if index < len(values) {
					value := values[index]
					index++
					if strings.HasPrefix(name, "DB") || strings.HasPrefix(name, "MB") || strings.HasPrefix(name, "EB") || strings.HasPrefix(name, "AB") {
						if tagType == SINT_TAG_TYPE {
							if rqst.Unsigned {
								buf[i] = byte(uint8(Utility.ToInt(value)))
							} else {
								buf[i] = byte(int8(Utility.ToInt(value)))
							}
						} else if tagType == INT_TAG_TYPE {
							val := Utility.ToInt(value)
							if rqst.Unsigned {
								helper.SetValueAt(buf, i, uint16(val))
							} else {
								helper.SetValueAt(buf, i, int16(val))
							}
						} else if tagType == DINT_TAG_TYPE {
							val := Utility.ToInt(value)
							if rqst.Unsigned {
								helper.SetValueAt(buf, i, uint32(val))
							} else {
								helper.SetValueAt(buf, i, int32(val))
							}
						} else if tagType == LINT_TAG_TYPE {
							val := Utility.ToInt(value)
							if rqst.Unsigned {
								helper.SetValueAt(buf, i, uint64(val))
							} else {
								helper.SetValueAt(buf, i, int64(val))
							}
						} else if tagType == REAL_TAG_TYPE {
							val := Utility.ToNumeric(value)
							helper.SetRealAt(buf, i, float32(val))
						} else if tagType == LREAL_TAG_TYPE {
							val := Utility.ToNumeric(value)
							helper.SetLRealAt(buf, i, float64(val))
						} else {
							return nil, status.Errorf(
								codes.Internal,
								Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Tag type not found!")))
						}
					} else if strings.HasPrefix(name, "T") {

						// The values must be a unix time.
						helper.SetDateTimeAt(buf, i, time.Unix(int64(value.(float64)), 0))
					} else if strings.HasPrefix(name, "C") {
						fmt.Println("---> CT not implemented!")

						//helper.SetCounterAt()
					}
				}
			}
		}

		// write the value to the plc.
		if strings.HasPrefix(name, "DB") || strings.HasPrefix(name, "MB") || strings.HasPrefix(name, "EB") || strings.HasPrefix(name, "AB") {
			//Write data blocks from PLC
			if tagType == BOOL_TAG_TYPE {
				// recalculate values for boolean...
				offset := int(offset / 8)
				err = c.client.AGWriteDB(number, offset, len(buf), buf)
			} else {
				err = c.client.AGWriteDB(number, offset, size*length, buf)
			}
			if err != nil {
				fmt.Println("line 575: write value: at address ", number, "offset", offset, "size", size*length, " buffer size ", len(buf))
				fmt.Println("line 576: error ", err)
			}
		} else if strings.HasPrefix(name, "T") {
			//Write timer from PLC
			err = c.client.AGWriteTM(number, size*length, buf)
		} else if strings.HasPrefix(name, "C") {
			//Write counter from PLC
			err = c.client.AGWriteCT(number, size*length, buf)
		}

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

	} else {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No connection found with id "+rqst.ConnectionId)))
	}

	return &plcpb.WriteTagRsp{
		Result: true,
	}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "plc_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	//log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The first argument must be the port number to listen to.
	port := defaultPort // the default value.

	if len(os.Args) > 1 {
		port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(plcpb.File_plc_plcpb_plc_proto.Services().Get(0).FullName())
	s_impl.Proto = plcpb.File_plc_plcpb_plc_proto.Path()
	s_impl.Path, _ = os.Executable()
	package_ := string(plcpb.File_plc_plcpb_plc_proto.Package().Name())
	s_impl.Path = s_impl.Path[strings.Index(s_impl.Path, package_):]
	s_impl.Port = port
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.Connections = make([]connection, 0)
	s_impl.connections = make(map[string]*connection)
	s_impl.Permissions = make([]interface{}, 0)

	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Here I will retreive the list of connections from file if there are some...
	s_impl.init()

	// First of all I will creat a listener.
	// Create the channel to listen on
	lis, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		fmt.Println("could not list on %s: %s", s_impl.Domain, err)
		return
	}

	var grpcServer *grpc.Server
	if s_impl.TLS {
		// Load the certificates from disk
		certificate, err := tls.LoadX509KeyPair(s_impl.CertFile, s_impl.KeyFile)
		if err != nil {
			fmt.Println("could not load server key pair: %s", err)
			return
		}

		// Create a certificate pool from the certificate authority
		certPool := x509.NewCertPool()
		ca, err := ioutil.ReadFile(s_impl.CertAuthorityTrust)
		if err != nil {
			fmt.Println("could not read ca certificate: %s", err)
			return
		}

		// Append the client certificates from the CA
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			fmt.Println("failed to append client certs")
			return
		}

		// Create the TLS credentials
		creds := credentials.NewTLS(&tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
		})

		// Create the gRPC server with the credentials
		opts := []grpc.ServerOption{grpc.Creds(creds), grpc.UnaryInterceptor(Interceptors.ServerUnaryInterceptor), grpc.StreamInterceptor(Interceptors.ServerStreamInterceptor)}
		grpcServer = grpc.NewServer(opts...)

	} else {
		grpcServer = grpc.NewServer([]grpc.ServerOption{grpc.UnaryInterceptor(Interceptors.ServerUnaryInterceptor), grpc.StreamInterceptor(Interceptors.ServerStreamInterceptor)}...)
	}

	plcpb.RegisterPlcServiceServer(grpcServer, s_impl)
	reflection.Register(grpcServer)
	// Here I will make a signal hook to interrupt to exit cleanly.
	go func() {
		fmt.Println(s_impl.Name + " grpc service is starting")
		// no web-rpc server.
		if err := grpcServer.Serve(lis); err != nil {

			if err.Error() == "signal: killed" {
				fmt.Println("service ", s_impl.Name, " was stop!")
			}
		}

		for _, c := range s_impl.connections {
			// Create the connection with the plc.
			c.handler.Close()
		}
		fmt.Println(s_impl.Name + " grpc service is closed")
	}()

	// Wait for signal to stop.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

}