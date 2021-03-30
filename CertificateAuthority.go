package main

import (
	"context"
	"os"

	"os/exec"

	"io/ioutil"
	"log"
	"net"

	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/Globular/Interceptors"
	"github.com/globulario/services/golang/ca/capb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (self *Globule) startCertificateAuthorityService() error {
	// The Certificate Authority
	id := string(capb.File_proto_ca_proto.Services().Get(0).FullName())
	certificate_authority_server, port, err := self.startInternalService(id, capb.File_proto_ca_proto.Path(), false, Interceptors.ServerUnaryInterceptor, Interceptors.ServerStreamInterceptor)

	if err == nil {
		self.inernalServices = append(self.inernalServices, certificate_authority_server)

		// Create the channel to listen on admin port.
		lis, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
		if err != nil {
			log.Fatalf("could not certificate authority signing  service %s: %s", self.Name, err)
		}

		capb.RegisterCertificateAuthorityServer(certificate_authority_server, self)
		// Here I will make a signal hook to interrupt to exit cleanly.

		go func() {
			// no web-rpc server.
			if err := certificate_authority_server.Serve(lis); err != nil {
				log.Println(err)
			}
			// Close it proxy process
			s := self.getService(id)
			pid := getIntVal(s, "ProxyProcess")
			Utility.TerminateProcess(pid, 0)
			s.Store("ProxyProcess", -1)
			self.saveConfig()
		}()

	}
	return err
}

func (self *Globule) signCertificate(client_csr string) (string, error) {

	// first of all I will save the incomming file into a temporary file...
	client_csr_path := os.TempDir() + "/" + Utility.RandomUUID()
	err := ioutil.WriteFile(client_csr_path, []byte(client_csr), 0644)
	if err != nil {
		return "", err

	}

	client_crt_path := os.TempDir() + "/" + Utility.RandomUUID()

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "x509")
	args = append(args, "-req")
	args = append(args, "-passin")
	args = append(args, "pass:"+self.CertPassword)
	args = append(args, "-days")
	args = append(args, Utility.ToString(self.CertExpirationDelay))
	args = append(args, "-in")
	args = append(args, client_csr_path)
	args = append(args, "-CA")
	args = append(args, self.creds+"/"+"ca.crt") // use certificate
	args = append(args, "-CAkey")
	args = append(args, self.creds+"/"+"ca.key") // and private key to sign the incommin csr
	args = append(args, "-set_serial")
	args = append(args, "01")
	args = append(args, "-out")
	args = append(args, client_crt_path)
	args = append(args, "-extfile")
	args = append(args, self.creds+"/"+"san.conf")
	args = append(args, "-extensions")
	args = append(args, "v3_req")
	err = exec.Command(cmd, args...).Run()
	if err != nil {
		return "", err
	}

	// I will read back the crt file.
	client_crt, err := ioutil.ReadFile(client_crt_path)

	// remove the tow temporary files.
	defer os.Remove(client_crt_path)
	defer os.Remove(client_csr_path)

	if err != nil {
		return "", err
	}

	return string(client_crt), nil

}

///////////////////////////////////// API //////////////////////////////////////

// Signed certificate request (CSR)
// csr: Take a certificate signing request as input and sing it with the ca cetificate.
// crt: The a client certificate is return.
// err: The error is generated by openSSL.
func (self *Globule) SignCertificate(ctx context.Context, rqst *capb.SignCertificateRequest) (*capb.SignCertificateResponse, error) {

	client_crt, err := self.signCertificate(rqst.Csr)

	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	return &capb.SignCertificateResponse{
		Crt: client_crt,
	}, nil
}

// Retunr the ca certificate.
// ca_crt: Return the Authority Trust Certificate. (ca.crt)
func (self *Globule) GetCaCertificate(ctx context.Context, rqst *capb.GetCaCertificateRequest) (*capb.GetCaCertificateResponse, error) {

	ca_crt, err := ioutil.ReadFile(self.creds + "/" + "ca.crt")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &capb.GetCaCertificateResponse{
		Ca: string(ca_crt),
	}, nil
}
