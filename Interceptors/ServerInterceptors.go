package Interceptors

// TODO for the validation, use a map to store valid method/token/ressource/access
// the validation will be renew only if the token expire. And when a token expire
// the value in the map will be discard. That way it will put less charge on the server
// side.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/davecourtois/Globular/file/filepb"
	"github.com/davecourtois/Globular/lb"

	"github.com/davecourtois/Globular/api"
	"github.com/davecourtois/Globular/lb/lbpb"
	"github.com/davecourtois/Globular/ressource"
	"github.com/davecourtois/Globular/storage/storage_store"
	"github.com/davecourtois/Utility"

	"github.com/shirou/gopsutil/load"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	// The ressource client
	ressource_client *ressource.Ressource_Client

	// The load balancer client.
	lb_client *lb.Lb_Client

	// The map will contain connection with other server of same kind to load
	// balance the server charge.
	clients map[string]api.Client

	// That will contain the permission in memory to limit the number
	// of ressource request...
	cache *storage_store.BigCache_store
)

/**
 * Get a the local ressource client.
 */
func getLoadBalancingClient(domain string, serverId string, serviceName string, serverDomain string, serverPort int32) (*lb.Lb_Client, error) {

	var err error
	if lb_client == nil {
		lb_client, err = lb.NewLb_Client(domain, "lb.LoadBalancingService")
		if err != nil {
			return nil, err
		}

		fmt.Println("----> start load balancing for ", serviceName)

		// Here I will create the client map.
		clients = make(map[string]api.Client)

		// Now I will start reporting load at each minutes.
		ticker := time.NewTicker(1 * time.Minute)
		go func() {
			for {
				select {
				case <-ticker.C:
					stats, err := load.Avg()
					if err != nil {
						break
					}
					load_info := &lbpb.LoadInfo{
						ServerInfo: &lbpb.ServerInfo{
							Id:     serverId,
							Name:   serviceName,
							Domain: serverDomain,
							Port:   serverPort,
						},
						Load1:  stats.Load1,
						Load5:  stats.Load5,
						Load15: stats.Load15,
					}

					lb_client.ReportLoadInfo(load_info)
				}
			}
		}()
	}

	return lb_client, nil
}

/**
 * Get a the local ressource client.
 */
func getRessourceClient(domain string) (*ressource.Ressource_Client, error) {
	var err error
	if ressource_client == nil {
		ressource_client, err = ressource.NewRessource_Client(domain, "ressource.RessourceService")
		if err != nil {
			return nil, err
		}
	}

	return ressource_client, nil
}

/**
 * A singleton use to access the cache.
 */
func getCache() *storage_store.BigCache_store {
	if cache == nil {
		cache = storage_store.NewBigCache_store()
		err := cache.Open("")
		if err != nil {
			fmt.Println(err)
		}
	}
	return cache
}

/**
 * Validate user file permission.
 */
func ValidateUserRessourceAccess(domain string, token string, method string, path string, permission int32) error {

	// keep the values in the map for the lifetime of the token and validate it
	// from local map.
	ressource_client, err := getRessourceClient(domain)
	if err != nil {
		return err
	}

	// get access from remote source.
	hasAccess, err := ressource_client.ValidateUserRessourceAccess(token, path, method, permission)
	if err != nil {
		return err
	}

	if !hasAccess {
		user, _, _, _ := ValidateToken(token)
		return errors.New("Permission denied for user " + user + " to execute methode " + method + " on ressource " + path)
	}

	return nil
}

/**
 * Validate application ressource permission.
 */
func ValidateApplicationRessourceAccess(domain string, applicationName string, method string, path string, permission int32) error {

	// keep the values in the map for the lifetime of the token and validate it
	// from local map.
	ressource_client, err := getRessourceClient(domain)
	if err != nil {
		return err
	}

	// get access from remote source.
	hasAccess, err := ressource_client.ValidateApplicationRessourceAccess(applicationName, path, method, permission)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("Permission denied for application " + applicationName + " to execute method " + method + " on ressource " + path)
	}

	return nil
}

func ValidateUserAccess(domain string, token string, method string) (bool, error) {
	clientId, _, expire, err := ValidateToken(token)

	key := Utility.GenerateUUID(clientId + method)
	if err != nil || expire < time.Now().Unix() {
		getCache().RemoveItem(key)
	}

	_, err = getCache().GetItem(key)
	if err == nil {
		// Here a value exist in the store...
		return true, nil
	}

	ressource_client, err := getRessourceClient(domain)
	if err != nil {
		return false, err
	}

	// get access from remote source.
	hasAccess, err := ressource_client.ValidateUserAccess(token, method)

	if hasAccess {
		getCache().SetItem(key, []byte(""))
	}

	return hasAccess, err
}

/**
 * Validate Application method access.
 */
func ValidateApplicationAccess(domain string, application string, method string) (bool, error) {
	key := Utility.GenerateUUID(application + method)

	_, err := getCache().GetItem(key)
	if err == nil {
		// Here a value exist in the store...
		return true, nil
	}

	ressource_client, err := getRessourceClient(domain)
	if err != nil {
		return false, err
	}

	// get access from remote source.
	hasAccess, err := ressource_client.ValidateApplicationAccess(application, method)
	if hasAccess {
		getCache().SetItem(key, []byte(""))

		// Here I will set a timeout for the permission.
		timeout := time.NewTimer(15 * time.Minute)
		go func() {
			<-timeout.C
			getCache().RemoveItem(key)
		}()
	}
	return hasAccess, err
}

// Refresh a token.
func refreshToken(domain string, token string) (string, error) {
	ressource_client, err := getRessourceClient(domain)
	if err != nil {
		return "", err
	}

	return ressource_client.RefreshToken(token)
}

// That interceptor is use by all services except the ressource service who has
// it own interceptor.
func ServerUnaryInterceptor(ctx context.Context, rqst interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	// The token and the application id.
	var token string
	var application string
	var path string
	var domain string // This is the target domain, the one use in TLS certificate.

	var load_balanced bool

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
		// in case of ressource path.
		path = strings.Join(md["path"], "")
		domain = strings.Join(md["domain"], "")

		load_balanced_ := strings.Join(md["load_balanced"], "")
		ctx = metadata.AppendToOutgoingContext(ctx, "load_balanced", "") // Set back the value to nothing.
		load_balanced = load_balanced_ == "true"
	}

	// Here I will test if the
	method := info.FullMethod

	if len(domain) == 0 {
		return nil, errors.New("No domain was given for method call '" + method + "'")
	}

	// If the call come from a local client it has hasAccess
	hasAccess := false // strings.HasPrefix(ip, "127.0.0.1") || strings.HasPrefix(ip, Utility.MyIP())

	// needed to get access to the system.
	if method == "/admin.AdminService/GetConfig" ||
		method == "/services.ServiceDiscovery/FindServices" ||
		method == "/services.ServiceDiscovery/FindServices/GetServiceDescriptor" ||
		method == "/services.ServiceDiscovery/FindServices/GetServicesDescriptor" ||
		method == "/services.ServiceDiscovery/FindServices/GetServicesDescriptor" ||
		method == "/dns.DnsService/GetA" || method == "/dns.DnsService/GetAAAA" ||
		method == "/ressource.RessourceService/Log" {
		hasAccess = true
	}

	var clientId string
	var err error

	if len(token) > 0 {
		clientId, _, _, err = ValidateToken(token)
		if err != nil {
			return nil, err
		}
	}

	// Test if the application has access to execute the method.
	if len(application) > 0 && !hasAccess {
		hasAccess, _ = ValidateApplicationAccess(domain, application, method)
	}

	// Test if the user has access to execute the method
	if len(token) > 0 && !hasAccess {
		hasAccess, _ = ValidateUserAccess(domain, token, method)
	}

	// Connect to the ressource services for the given domain.
	ressource_client, err := getRessourceClient(domain)
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		err := errors.New("Permission denied to execute method " + method + " user:" + clientId + " domain:" + domain + " application:" + application)
		fmt.Println(err)
		ressource_client.Log(application, clientId, method, err)
		return nil, err
	}

	// Now I will test file permission.
	if clientId != "sa" {

		// Here I will retreive the permission from the database if there is some...
		// the path will be found in the parameter of the method.
		permission, err := ressource_client.GetActionPermission(method)
		if err == nil && permission != -1 {
			// I will test if the user has file permission.
			err = ValidateUserRessourceAccess(domain, token, method, path, permission)
			if err != nil {
				if len(application) == 0 {
					return nil, err
				}
				err = ValidateApplicationRessourceAccess(domain, application, method, path, permission)
				if err != nil {
					return nil, err
				}
			}

		}

	}

	// Here I will exclude local service from the load balancing.
	var candidates []*lbpb.ServerInfo
	// I will try to get the list of candidates for load balancing
	if Utility.GetProperty(info.Server, "Port") != nil {

		// Here I will refresh the load balance of the server to keep track of real load.
		lb_client, err := getLoadBalancingClient(domain, Utility.GetProperty(info.Server, "Id").(string), Utility.GetProperty(info.Server, "Name").(string), Utility.GetProperty(info.Server, "Domain").(string), int32(Utility.GetProperty(info.Server, "Port").(int)))
		if err != nil {
			return nil, err
		}

		stats, _ := load.Avg()
		load_info := &lbpb.LoadInfo{
			ServerInfo: &lbpb.ServerInfo{
				Id:     Utility.GetProperty(info.Server, "Id").(string),
				Name:   Utility.GetProperty(info.Server, "Name").(string),
				Domain: Utility.GetProperty(info.Server, "Domain").(string),
				Port:   int32(Utility.GetProperty(info.Server, "Port").(int)),
			},
			Load1:  stats.Load1,
			Load5:  stats.Load5,
			Load15: stats.Load15,
		}
		lb_client.ReportLoadInfo(load_info)

		// if load balanced is false I will get list of candidate.
		if load_balanced == false {
			candidates, _ = lb_client.GetCandidates(Utility.GetProperty(info.Server, "Name").(string))
		}

	}

	var result interface{}

	// Execute the action.
	if candidates != nil {
		serverId := Utility.GetProperty(info.Server, "Id").(string)
		// Here there is some candidate in the list.
		for i := 0; i < len(candidates); i++ {
			candidate := candidates[i]

			if candidate.GetId() == serverId {
				// In that case the handler is the actual server.
				result, err = handler(ctx, rqst)
				fmt.Println("398 execute load balanced request ", serverId)
				break // stop the loop...
			} else {
				// Here the canditade is the actual server so I will dispatch the request to the candidate.
				if clients[candidate.GetId()] == nil {
					// Here I will create an instance of the client.
					newClientFct := method[1:strings.Index(method, ".")]
					newClientFct = "New" + strings.ToUpper(newClientFct[0:1]) + newClientFct[1:] + "_Client"
					fmt.Println(newClientFct)
					// Here I will create a connection with the other server in order to be able to dispatch the request.
					results, err := Utility.CallFunction(newClientFct, candidate.GetDomain(), candidate.GetId())
					if err != nil {
						fmt.Println(err)
						continue // skip to the next client.
					}
					// So here I will keep the client inside the map.
					clients[candidate.GetId()] = results[0].Interface().(api.Client)
				}
				fmt.Println("416 redirect rqst from ", serverId, " to ", candidate.GetId())
				// Here I will invoke the request on the server whit the same context, so permission and token etc will be kept the save.
				result, err = clients[candidate.GetId()].Invoke(method, rqst, metadata.AppendToOutgoingContext(ctx, "load_balanced", "true", "domain", Utility.GetProperty(info.Server, "Domain").(string), "path", path, "application", application, "token", token))
				if err != nil {
					fmt.Println(err)
					continue // skip to the next client.
				} else {
					break
				}
			}
		}

	} else {
		if Utility.GetProperty(info.Server, "Id") != nil {
			fmt.Println("421 execute request ", Utility.GetProperty(info.Server, "Id").(string))
		}
		result, err = handler(ctx, rqst)
	}

	// Send log event...
	if (len(application) > 0 && len(clientId) > 0 && clientId != "sa") || err != nil {
		ressource_client.Log(application, clientId, method, err)
	}

	// Here depending of the request I will execute more actions.
	if err == nil {
		if method == "/file.FileService/CreateDir" && clientId != "sa" {
			rqst := rqst.(*filepb.CreateDirRequest)
			err := ressource_client.CreateDirPermissions(token, rqst.GetPath(), rqst.GetName())
			if err != nil {
				fmt.Println(err)
				return nil, err
			}

			// Here I will set the ressource owner for the directory.
			if strings.HasSuffix(rqst.GetPath(), "/") {
				ressource_client.SetRessourceOwner(clientId, rqst.GetPath()+rqst.GetName(), "")
			} else {
				ressource_client.SetRessourceOwner(clientId, rqst.GetPath()+"/"+rqst.GetName(), "")
			}

		} else if method == "/file.FileService/Rename" {
			rqst := rqst.(*filepb.RenameRequest)
			err := ressource_client.RenameFilePermission(rqst.GetPath(), rqst.GetOldName(), rqst.GetNewName())
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		} else if method == "/file.FileService/DeleteFile" {
			rqst := rqst.(*filepb.DeleteFileRequest)
			err := ressource_client.DeleteFilePermissions(rqst.GetPath())
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		} else if method == "/file.FileService/DeleteDir" {
			rqst := rqst.(*filepb.DeleteDirRequest)
			err := ressource_client.DeleteDirPermissions(rqst.GetPath())
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		}
	}

	return result, err

}

// Stream interceptor.
func ServerStreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	// The token and the application id.
	var token string
	var application string
	var domain string
	var path string
	//var ip string

	//var mac string
	//Get the caller ip address.
	//p, _ := peer.FromContext(stream.Context())
	//ip = p.Addr.String()

	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
		path = strings.Join(md["path"], "")
		//mac = strings.Join(md["mac"], "")
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		domain = strings.Join(md["domain"], "")
	}

	method := info.FullMethod

	var clientId string
	var err error

	if len(token) > 0 {
		clientId, _, _, err = ValidateToken(token)
		if err != nil {
			return err
		}
	}

	ressource_client, err := getRessourceClient(domain)
	if err != nil {
		return err
	}

	// If the call come from a local client it has hasAccess
	hasAccess := false // strings.HasPrefix(p.Addr.String(), "127.0.0.1") || strings.HasPrefix(ip, Utility.MyIP())
	// needed by the admin.
	if application == "admin" ||
		method == "/services.ServiceDiscovery/FindServices/GetServicesDescriptor" {
		hasAccess = true
	}

	// Test if the user has access to execute the method
	if len(token) > 0 && !hasAccess {
		hasAccess, _ = ValidateUserAccess(domain, token, method)
	}

	// Test if the application has access to execute the method.
	if len(application) > 0 && !hasAccess {
		hasAccess, _ = ValidateApplicationAccess(domain, application, method)
	}

	// Return here if access is denied.
	if !hasAccess {
		return errors.New("Permission denied to execute method " + method)
	}

	// Now the permissions
	if len(path) > 0 && clientId != "sa" {
		permission, err := ressource_client.GetActionPermission(method)
		if err == nil && permission != -1 {
			// I will test if the user has file permission.
			err = ValidateUserRessourceAccess(domain, token, method, path, permission)
			if err != nil {
				if len(application) == 0 {
					return err
				}
				err = ValidateApplicationRessourceAccess(domain, application, method, path, permission)
				if err != nil {
					return err
				}
			}

		}
	} else if clientId != "sa" {
		permission, err := ressource_client.GetActionPermission(method)
		if err == nil && permission != -1 {
			return errors.New("Permission denied to execute method " + method)
		}
	}

	err = handler(srv, stream)

	// TODO find when the stream is closing and log only one time.
	//if err == io.EOF {
	// Send log event...
	ressource_client.Log(application, clientId, method, err)
	//}

	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}
