package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"

	//	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func (self *Globule) startRbacService() error {
	id := string(rbacpb.File_proto_rbac_proto.Services().Get(0).FullName())
	rbac_server, err := self.startInternalService(id, rbacpb.File_proto_rbac_proto.Path(), self.RbacPort, self.RbacProxy, self.Protocol == "https", self.unaryResourceInterceptor, self.streamResourceInterceptor)
	if err == nil {
		self.inernalServices = append(self.inernalServices, rbac_server)

		// Create the channel to listen on resource port.
		lis, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(self.RbacPort))
		if err != nil {
			log.Fatalf("could not start resource service %s: %s", self.getDomain(), err)
		}

		rbacpb.RegisterRbacServiceServer(rbac_server, self)

		// Here I will make a signal hook to interrupt to exit cleanly.
		go func() {
			// no web-rpc server.
			if err = rbac_server.Serve(lis); err != nil {
				log.Println(err)
			}
			s := self.getService(id)
			pid := getIntVal(s, "ProxyProcess")
			Utility.TerminateProcess(pid, 0)
			s.Store("ProxyProcess", -1)
			self.saveConfig()
		}()
	}

	return err
}

func (self *Globule) setEntityResourcePermissions(entity string, path string) error {
	// Here I will retreive the actual list of paths use by this user.
	data, err := self.permissions.GetItem(entity)
	paths := make([]string, 0)

	if err == nil {
		err := json.Unmarshal(data, &paths)
		if err != nil {
			return err
		}
	}

	if !Utility.Contains(paths, path) {
		paths = append(paths, path)
	} else {
		return nil // nothing todo here...
	}

	// simply marshal the permission and put it into the store.
	data, err = json.Marshal(paths)
	if err != nil {
		return err
	}
	return self.permissions.SetItem(entity, data)
}

func (self *Globule) setResourcePermissions(path string, permissions *rbacpb.Permissions) error {

	// First of all I need to remove the existing permission.
	self.deleteResourcePermissions(path, permissions)

	// Allowed resources
	allowed := permissions.Allowed
	if allowed != nil {
		for i := 0; i < len(allowed); i++ {

			// Accounts
			for j := 0; j < len(allowed[i].Accounts); j++ {

				err := self.setEntityResourcePermissions(allowed[i].Accounts[j], path)
				if err != nil {
					return err
				}
			}

			// Groups
			for j := 0; j < len(allowed[i].Groups); j++ {
				err := self.setEntityResourcePermissions(allowed[i].Groups[j], path)
				if err != nil {
					return err
				}
			}

			// Organizations
			for j := 0; j < len(allowed[i].Organizations); j++ {
				err := self.setEntityResourcePermissions(allowed[i].Organizations[j], path)
				if err != nil {
					return err
				}
			}

			// Applications
			for j := 0; j < len(allowed[i].Applications); j++ {
				err := self.setEntityResourcePermissions(allowed[i].Applications[j], path)
				if err != nil {
					return err
				}
			}

			// Peers
			for j := 0; j < len(allowed[i].Peers); j++ {
				err := self.setEntityResourcePermissions(allowed[i].Peers[j], path)
				if err != nil {
					return err
				}
			}
		}
	}

	// Denied resources
	denied := permissions.Denied
	if denied != nil {
		for i := 0; i < len(denied); i++ {
			// Acccounts
			for j := 0; j < len(denied[i].Accounts); j++ {
				err := self.setEntityResourcePermissions(denied[i].Accounts[j], path)
				if err != nil {
					return err
				}
			}
			// Applications
			for j := 0; j < len(denied[i].Applications); j++ {
				err := self.setEntityResourcePermissions(denied[i].Applications[j], path)
				if err != nil {
					return err
				}
			}

			// Peers
			for j := 0; j < len(denied[i].Peers); j++ {
				err := self.setEntityResourcePermissions(denied[i].Peers[j], path)
				if err != nil {
					return err
				}
			}

			// Groups
			for j := 0; j < len(denied[i].Groups); j++ {
				err := self.setEntityResourcePermissions(denied[i].Groups[j], path)
				if err != nil {
					return err
				}
			}

			// Organizations
			for j := 0; j < len(denied[i].Organizations); j++ {
				err := self.setEntityResourcePermissions(denied[i].Organizations[j], path)
				if err != nil {
					return err
				}
			}

		}
	}

	// Owned resources
	owners := permissions.Owners
	if owners != nil {
		// Acccounts
		if owners.Accounts != nil {
			for j := 0; j < len(owners.Accounts); j++ {
				err := self.setEntityResourcePermissions(owners.Accounts[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				err := self.setEntityResourcePermissions(owners.Applications[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Peers
		if owners.Peers != nil {
			for j := 0; j < len(owners.Peers); j++ {
				err := self.setEntityResourcePermissions(owners.Peers[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := 0; j < len(owners.Groups); j++ {
				err := self.setEntityResourcePermissions(owners.Groups[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				err := self.setEntityResourcePermissions(owners.Organizations[j], path)
				if err != nil {
					return err
				}
			}
		}
	}
	// simply marshal the permission and put it into the store.
	data, err := json.Marshal(permissions)
	if err != nil {
		return err
	}

	err = self.permissions.SetItem(path, data)
	if err != nil {
		return err
	}

	return nil
}

//* Set resource permissions this method will replace existing permission at once *
func (self *Globule) SetResourcePermissions(ctx context.Context, rqst *rbacpb.SetResourcePermissionsRqst) (*rbacpb.SetResourcePermissionsRqst, error) {
	err := self.setResourcePermissions(rqst.Path, rqst.Permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.SetResourcePermissionsRqst{}, nil
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

/**
 * Remove a resource path for an entity.
 */
func (self *Globule) deleteEntityResourcePermissions(entity string, path string) error {
	data, err := self.permissions.GetItem(entity)
	if err != nil {
		return err
	}

	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return err
	}

	// Here I will
	if Utility.Contains(paths, path) {
		paths = remove(paths, path)
		data, err = json.Marshal(paths)
		if err != nil {
			return err
		}

		return self.permissions.SetItem(entity, data)
	}

	return nil
}

// Remouve a ressource permission
func (self *Globule) deleteResourcePermissions(path string, permissions *rbacpb.Permissions) error {

	// Allowed resources
	allowed := permissions.Allowed
	if allowed != nil {
		for i := 0; i < len(allowed); i++ {

			// Accounts
			for j := 0; j < len(allowed[i].Accounts); j++ {
				err := self.deleteEntityResourcePermissions(allowed[i].Accounts[j], path)
				if err != nil {
					return err
				}
			}

			// Groups
			for j := 0; j < len(allowed[i].Groups); j++ {
				err := self.deleteEntityResourcePermissions(allowed[i].Groups[j], path)
				if err != nil {
					return err
				}
			}

			// Organizations
			for j := 0; j < len(allowed[i].Organizations); j++ {
				err := self.deleteEntityResourcePermissions(allowed[i].Organizations[j], path)
				if err != nil {
					return err
				}
			}

			// Applications
			for j := 0; j < len(allowed[i].Applications); j++ {
				err := self.deleteEntityResourcePermissions(allowed[i].Applications[j], path)
				if err != nil {
					return err
				}
			}

			// Peers
			for j := 0; j < len(allowed[i].Peers); j++ {
				err := self.deleteEntityResourcePermissions(allowed[i].Peers[j], path)
				if err != nil {
					return err
				}
			}
		}
	}

	// Denied resources
	denied := permissions.Denied
	if denied != nil {
		for i := 0; i < len(denied); i++ {
			// Acccounts
			for j := 0; j < len(denied[i].Accounts); j++ {
				err := self.deleteEntityResourcePermissions(denied[i].Accounts[j], path)
				if err != nil {
					return err
				}
			}
			// Applications
			for j := 0; j < len(denied[i].Applications); j++ {
				err := self.deleteEntityResourcePermissions(denied[i].Applications[j], path)
				if err != nil {
					return err
				}
			}

			// Peers
			for j := 0; j < len(denied[i].Peers); j++ {
				err := self.deleteEntityResourcePermissions(denied[i].Peers[j], path)
				if err != nil {
					return err
				}
			}

			// Groups
			for j := 0; j < len(denied[i].Groups); j++ {
				err := self.deleteEntityResourcePermissions(denied[i].Groups[j], path)
				if err != nil {
					return err
				}
			}

			// Organizations
			for j := 0; j < len(denied[i].Organizations); j++ {
				err := self.deleteEntityResourcePermissions(denied[i].Organizations[j], path)
				if err != nil {
					return err
				}
			}
		}
	}

	// Owned resources
	owners := permissions.Owners

	if owners != nil {
		// Acccounts
		if owners.Accounts != nil {
			for j := 0; j < len(owners.Accounts); j++ {
				err := self.deleteEntityResourcePermissions(owners.Accounts[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				err := self.deleteEntityResourcePermissions(owners.Applications[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Peers
		if owners.Peers != nil {
			for j := 0; j < len(owners.Peers); j++ {
				err := self.deleteEntityResourcePermissions(owners.Peers[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := 0; j < len(owners.Groups); j++ {
				err := self.deleteEntityResourcePermissions(owners.Groups[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				err := self.deleteEntityResourcePermissions(owners.Organizations[j], path)
				if err != nil {
					return err
				}
			}
		}
	}

	return self.permissions.RemoveItem(path)

}

func (self *Globule) getResourcePermissions(path string) (*rbacpb.Permissions, error) {
	data, err := self.permissions.GetItem(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	permissions := new(rbacpb.Permissions)
	err = json.Unmarshal(data, &permissions)
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

//* Delete a resource permissions (when a resource is deleted) *
func (self *Globule) DeleteResourcePermissions(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionsRqst) (*rbacpb.DeleteResourcePermissionsRqst, error) {

	permissions, err := self.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = self.deleteResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionsRqst{}, nil
}

//* Delete a specific resource permission *
func (self *Globule) DeleteResourcePermission(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionRqst) (*rbacpb.DeleteResourcePermissionRqst, error) {

	permissions, err := self.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rqst.Type == rbacpb.PermissionType_ALLOWED {
		// Remove the permission from the allowed permission
		allowed := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Allowed); i++ {
			if permissions.Allowed[i].Name != rqst.Name {
				allowed = append(allowed, permissions.Allowed[i])
			}
		}
		permissions.Allowed = allowed
	} else if rqst.Type == rbacpb.PermissionType_DENIED {
		// Remove the permission from the allowed permission.
		denied := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Denied); i++ {
			if permissions.Denied[i].Name != rqst.Name {
				denied = append(denied, permissions.Denied[i])
			}
		}
		permissions.Denied = denied
	}
	err = self.setResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionRqst{}, nil
}

//* Get the ressource Permission.
func (self *Globule) GetResourcePermission(ctx context.Context, rqst *rbacpb.GetResourcePermissionRqst) (*rbacpb.GetResourcePermissionRsp, error) {
	permissions, err := self.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Search on allowed permission
	if rqst.Type == rbacpb.PermissionType_ALLOWED {
		for i := 0; i < len(permissions.Allowed); i++ {
			if permissions.Allowed[i].Name == rqst.Name {
				return &rbacpb.GetResourcePermissionRsp{Permission: permissions.Allowed[i]}, nil
			}
		}
	} else if rqst.Type == rbacpb.PermissionType_DENIED { // search in denied permissions.

		for i := 0; i < len(permissions.Denied); i++ {
			if permissions.Denied[i].Name == rqst.Name {
				return &rbacpb.GetResourcePermissionRsp{Permission: permissions.Allowed[i]}, nil
			}
		}
	}

	return nil, status.Errorf(
		codes.Internal,
		Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No permission found with name "+rqst.Name)))
}

//* Set specific resource permission  ex. read permission... *
func (self *Globule) SetResourcePermission(ctx context.Context, rqst *rbacpb.SetResourcePermissionRqst) (*rbacpb.SetResourcePermissionRsp, error) {
	permissions, err := self.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove the permission from the allowed permission
	if rqst.Type == rbacpb.PermissionType_ALLOWED {
		allowed := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Allowed); i++ {
			if permissions.Allowed[i].Name == rqst.Permission.Name {
				allowed = append(allowed, permissions.Allowed[i])
			} else {
				allowed = append(allowed, rqst.Permission)
			}
		}
		permissions.Allowed = allowed
	} else if rqst.Type == rbacpb.PermissionType_DENIED {

		// Remove the permission from the allowed permission.
		denied := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Denied); i++ {
			if permissions.Denied[i].Name == rqst.Permission.Name {
				denied = append(denied, permissions.Denied[i])
			} else {
				denied = append(denied, rqst.Permission)
			}
		}
		permissions.Denied = denied
	}
	err = self.setResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetResourcePermissionRsp{}, nil
}

//* Get resource permissions *
func (self *Globule) GetResourcePermissions(ctx context.Context, rqst *rbacpb.GetResourcePermissionsRqst) (*rbacpb.GetResourcePermissionsRsp, error) {
	permissions, err := self.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetResourcePermissionsRsp{Permissions: permissions}, nil
}

func (self *Globule) addResourceOwner(path string, subject string, subjectType rbacpb.SubjectType) error {
	permissions, err := self.getResourcePermissions(path)
	if err != nil {
		return err
	}

	// Owned resources
	owners := permissions.Owners
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		if !Utility.Contains(owners.Accounts, subject) {
			owners.Accounts = append(owners.Accounts, subject)
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		if !Utility.Contains(owners.Applications, subject) {
			owners.Applications = append(owners.Applications, subject)
		}
	} else if subjectType == rbacpb.SubjectType_GROUP {
		if !Utility.Contains(owners.Groups, subject) {
			owners.Groups = append(owners.Groups, subject)
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		if !Utility.Contains(owners.Organizations, subject) {
			owners.Organizations = append(owners.Organizations, subject)
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		if !Utility.Contains(owners.Peers, subject) {
			owners.Peers = append(owners.Peers, subject)
		}
	}
	permissions.Owners = owners
	err = self.setResourcePermissions(subject, permissions)
	if err != nil {
		return err
	}

	return nil
}

//* Add resource owner do nothing if it already exist
func (self *Globule) AddResourceOwner(ctx context.Context, rqst *rbacpb.AddResourceOwnerRqst) (*rbacpb.AddResourceOwnerRsp, error) {

	err := self.addResourceOwner(rqst.Path, rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.AddResourceOwnerRsp{}, nil
}

func (self *Globule) removeResourceOwner(owner string, subjectType rbacpb.SubjectType, path string) error {
	permissions, err := self.getResourcePermissions(path)
	if err != nil {
		return err
	}

	// Owned resources
	owners := permissions.Owners
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		if Utility.Contains(owners.Accounts, owner) {
			owners.Accounts = remove(owners.Accounts, owner)
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		if Utility.Contains(owners.Applications, owner) {
			owners.Applications = remove(owners.Applications, owner)
		}
	} else if subjectType == rbacpb.SubjectType_GROUP {
		if Utility.Contains(owners.Groups, owner) {
			owners.Groups = remove(owners.Groups, owner)
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		if Utility.Contains(owners.Organizations, owner) {
			owners.Organizations = remove(owners.Organizations, owner)
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		if Utility.Contains(owners.Peers, owner) {
			owners.Peers = remove(owners.Peers, owner)
		}
	}

	permissions.Owners = owners
	err = self.setResourcePermissions(path, permissions)
	if err != nil {
		return err
	}

	return nil
}

// Remove a Subject from denied list and allowed list.
func (self *Globule) removeResourceSubject(subject string, subjectType rbacpb.SubjectType, path string) error {
	permissions, err := self.getResourcePermissions(path)
	if err != nil {
		return err
	}

	// Allowed resources
	allowed := permissions.Allowed
	for i := 0; i < len(allowed); i++ {
		// Accounts
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			accounts := make([]string, 0)
			for j := 0; j < len(allowed[i].Accounts); j++ {
				if subject != allowed[i].Accounts[j] {
					accounts = append(accounts, allowed[i].Accounts[j])
				}

			}
			allowed[i].Accounts = accounts
		}

		// Groups
		if subjectType == rbacpb.SubjectType_GROUP {
			groups := make([]string, 0)
			for j := 0; j < len(allowed[i].Groups); j++ {
				if subject != allowed[i].Groups[j] {
					groups = append(groups, allowed[i].Groups[j])
				}
			}
			allowed[i].Groups = groups
		}

		// Organizations
		if subjectType == rbacpb.SubjectType_ORGANIZATION {
			organizations := make([]string, 0)
			for j := 0; j < len(allowed[i].Organizations); j++ {
				if subject != allowed[i].Organizations[j] {
					organizations = append(organizations, allowed[i].Organizations[j])
				}
			}
			allowed[i].Organizations = organizations
		}

		// Applications
		if subjectType == rbacpb.SubjectType_APPLICATION {
			applications := make([]string, 0)
			for j := 0; j < len(allowed[i].Applications); j++ {
				if subject != allowed[i].Applications[j] {
					applications = append(applications, allowed[i].Applications[j])
				}
			}
			allowed[i].Applications = applications
		}

		// Peers
		if subjectType == rbacpb.SubjectType_PEER {
			peers := make([]string, 0)
			for j := 0; j < len(allowed[i].Peers); j++ {
				if subject != allowed[i].Peers[j] {
					peers = append(peers, allowed[i].Peers[j])
				}
			}
			allowed[i].Peers = peers
		}
	}

	// Denied resources
	denied := permissions.Denied
	for i := 0; i < len(denied); i++ {
		// Accounts
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			accounts := make([]string, 0)
			for j := 0; j < len(denied[i].Accounts); j++ {
				if subject != denied[i].Accounts[j] {
					accounts = append(accounts, denied[i].Accounts[j])
				}

			}
			denied[i].Accounts = accounts
		}

		// Groups
		if subjectType == rbacpb.SubjectType_GROUP {
			groups := make([]string, 0)
			for j := 0; j < len(denied[i].Groups); j++ {
				if subject != denied[i].Groups[j] {
					groups = append(groups, denied[i].Groups[j])
				}
			}
			denied[i].Groups = groups
		}

		// Organizations
		if subjectType == rbacpb.SubjectType_ORGANIZATION {
			organizations := make([]string, 0)
			for j := 0; j < len(denied[i].Organizations); j++ {
				if subject != denied[i].Organizations[j] {
					organizations = append(organizations, denied[i].Organizations[j])
				}
			}
			denied[i].Organizations = organizations
		}

		// Applications
		if subjectType == rbacpb.SubjectType_APPLICATION {
			applications := make([]string, 0)
			for j := 0; j < len(denied[i].Applications); j++ {
				if subject != denied[i].Applications[j] {
					applications = append(applications, denied[i].Applications[j])
				}
			}
			denied[i].Applications = applications
		}

		// Peers
		if subjectType == rbacpb.SubjectType_PEER {
			peers := make([]string, 0)
			for j := 0; j < len(denied[i].Peers); j++ {
				if subject != denied[i].Peers[j] {
					peers = append(peers, denied[i].Peers[j])
				}
			}
			denied[i].Peers = peers
		}
	}

	err = self.setResourcePermissions(path, permissions)
	if err != nil {
		return err
	}

	return nil
}

//* Remove resource owner
func (self *Globule) RemoveResourceOwner(ctx context.Context, rqst *rbacpb.RemoveResourceOwnerRqst) (*rbacpb.RemoveResourceOwnerRsp, error) {
	err := self.removeResourceOwner(rqst.Subject, rqst.Type, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveResourceOwnerRsp{}, nil
}

//* That function must be call when a subject is removed to clean up permissions.
func (self *Globule) DeleteAllAccess(ctx context.Context, rqst *rbacpb.DeleteAllAccessRqst) (*rbacpb.DeleteAllAccessRsp, error) {

	// Here I must remove the subject from all permissions.
	data, err := self.permissions.GetItem(rqst.Subject)
	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	for i := 0; i < len(paths); i++ {

		// Remove from owner
		self.removeResourceOwner(rqst.Subject, rqst.Type, paths[i])

		// Remove from subject.
		self.removeResourceSubject(rqst.Subject, rqst.Type, paths[i])

	}

	err = self.permissions.RemoveItem(rqst.Subject)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteAllAccessRsp{}, nil
}

// Return  accessAllowed, accessDenied, error
func (self *Globule) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	// first I will test if permissions is define
	permissions, err := self.getResourcePermissions(path)
	if err != nil {
		// In that case I will try to get parent ressource permission.
		if len(strings.Split(path, "/")) > 1 {
			// test for it parent.
			log.Println("Evaluate the path ", path[0:strings.LastIndex(path, "/")])
			return self.validateAccess(subject, subjectType, name, path[0:strings.LastIndex(path, "/")])
		}

		if strings.Index(err.Error(), "leveldb: not found") != -1 {
			return true, false, err
		}
		// if no permission are define for a ressource anyone can access it.
		return false, false, err
	}

	log.Println("-----------------> 886 ", permissions)
	// Test if the Subject is owner of the ressource in that case I will git him access.
	owners := permissions.Owners
	isOwner := false
	subjectStr := ""
	if owners != nil {
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			subjectStr = "Account"
			if owners.Accounts != nil {
				if Utility.Contains(owners.Accounts, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_APPLICATION {
			subjectStr = "Application"
			if owners.Applications != nil {
				if Utility.Contains(owners.Applications, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_GROUP {
			subjectStr = "Group"
			if owners.Groups != nil {
				if Utility.Contains(owners.Groups, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
			subjectStr = "Organization"
			if owners.Organizations != nil {
				if Utility.Contains(owners.Organizations, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			subjectStr = "Peer"
			if owners.Peers != nil {
				if Utility.Contains(owners.Peers, subject) {
					isOwner = true
				}
			}
		}
	}

	// If the user is the owner no other validation are required.
	if isOwner {
		return true, false, nil
	}

	// First I will validate that the permission is not denied...
	var denied *rbacpb.Permission
	for i := 0; i < len(permissions.Denied); i++ {
		if permissions.Denied[i].Name == name {
			denied = permissions.Denied[i]
			break
		}
	}

	////////////////////// Test if the access is denied first. /////////////////////
	accessDenied := false

	// Here the Subject is not the owner...
	if denied != nil {
		if subjectType == rbacpb.SubjectType_ACCOUNT {

			// Here the subject is an account.
			if denied.Accounts != nil {
				accessDenied = Utility.Contains(denied.Accounts, subject)
			}

			// The access is not denied for the account itself, I will validate
			// that the account is not part of denied group.
			if !accessDenied {
				// I will test if one of the group account if part of hare access denied.
				p, err := self.getPersistenceStore()
				if err != nil {
					return false, false, err
				}

				// Here I will test if a newer token exist for that user if it's the case
				// I will not refresh that token.
				values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"_id":"`+subject+`"}`, ``)
				if err != nil {
					return false, false, errors.New("No account named " + subject + " exist!")
				}

				// from the account I will get the list of group.
				account := values.(map[string]interface{})
				if account["groups"] != nil {
					groups := []interface{}(account["groups"].(primitive.A))
					if groups != nil {
						for i := 0; i < len(groups); i++ {
							groupId := groups[i].(map[string]interface{})["$id"].(string)
							_, accessDenied_, _ := self.validateAccess(groupId, rbacpb.SubjectType_GROUP, name, path)
							if accessDenied_ {
								return false, true, errors.New("Access denied for " + subjectStr + " " + subject + "!")
							}
						}
					}
				}

				// from the account I will get the list of group.
				if account["organizations"] != nil {
					organizations := []interface{}(account["organizations"].(primitive.A))
					if organizations != nil {
						for i := 0; i < len(organizations); i++ {
							organizationId := organizations[i].(map[string]interface{})["$id"].(string)
							_, accessDenied_, _ := self.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
							if accessDenied_ {
								return false, true, errors.New("Access denied for " + subjectStr + " " + subject + "!")
							}
						}
					}
				}
			}

		} else if subjectType == rbacpb.SubjectType_APPLICATION {
			// Here the Subject is an application.
			if denied.Applications != nil {
				accessDenied = Utility.Contains(denied.Applications, subject)
			}
		} else if subjectType == rbacpb.SubjectType_GROUP {
			// Here the Subject is a group
			if denied.Groups != nil {
				accessDenied = Utility.Contains(denied.Groups, subject)
			}

			// The access is not denied for the account itself, I will validate
			// that the account is not part of denied group.
			if !accessDenied {
				// I will test if one of the group account if part of hare access denied.
				p, err := self.getPersistenceStore()
				if err != nil {
					return false, false, err
				}

				// Here I will test if a newer token exist for that user if it's the case
				// I will not refresh that token.
				values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+subject+`"}`, ``)
				if err != nil {
					return false, false, errors.New("No account named " + subject + " exist!")
				}

				// from the account I will get the list of group.
				group := values.(map[string]interface{})
				if group["organizations"] != nil {
					organizations := []interface{}(group["organizations"].(primitive.A))
					if organizations != nil {
						for i := 0; i < len(organizations); i++ {
							organizationId := organizations[i].(map[string]interface{})["$id"].(string)
							_, accessDenied_, _ := self.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
							if accessDenied_ {
								return false, true, errors.New("Access denied for " + subjectStr + " " + organizationId + "!")
							}
						}
					}
				}
			}

		} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
			// Here the Subject is an Organisations.
			if denied.Organizations != nil {
				accessDenied = Utility.Contains(denied.Organizations, subject)
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			// Here the Subject is a Peer.
			if denied.Peers != nil {
				accessDenied = Utility.Contains(denied.Peers, subject)
			}
		}
	}

	if accessDenied {
		err := errors.New("Access denied for " + subjectStr + " " + subject + "!")
		return false, true, err
	}

	var allowed *rbacpb.Permission
	for i := 0; i < len(permissions.Allowed); i++ {
		if permissions.Allowed[i].Name == name {
			allowed = permissions.Allowed[i]
			break
		}
	}

	hasAccess := false
	if allowed != nil {
		// Test if the access is allowed
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			if allowed.Accounts != nil {
				hasAccess = Utility.Contains(allowed.Accounts, subject)
				if hasAccess {
					return true, false, nil
				}
			}
			if !hasAccess {
				// I will test if one of the group account if part of hare access denied.
				p, err := self.getPersistenceStore()
				if err != nil {
					return false, false, err
				}

				// Here I will test if a newer token exist for that user if it's the case
				// I will not refresh that token.
				values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"_id":"`+subject+`"}`, ``)
				if err == nil {
					// from the account I will get the list of group.
					account := values.(map[string]interface{})
					if account["groups"] != nil {
						groups := []interface{}(account["groups"].(primitive.A))
						if groups != nil {
							for i := 0; i < len(groups); i++ {
								groupId := groups[i].(map[string]interface{})["$id"].(string)
								hasAccess_, _, _ := self.validateAccess(groupId, rbacpb.SubjectType_GROUP, name, path)
								if hasAccess_ {
									return true, false, nil
								}
							}
						}
					}

					// from the account I will get the list of group.
					if account["organizations"] != nil {
						organizations := []interface{}(account["organizations"].(primitive.A))
						if organizations != nil {
							for i := 0; i < len(organizations); i++ {
								organizationId := organizations[i].(map[string]interface{})["$id"].(string)
								hasAccess_, _, _ := self.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
								if hasAccess_ {
									return true, false, nil
								}
							}
						}
					}
				}
			}

		} else if subjectType == rbacpb.SubjectType_GROUP {
			// validate the group access
			if allowed.Groups != nil {
				hasAccess = Utility.Contains(allowed.Groups, subject)
				if hasAccess {
					return true, false, nil
				}
			}

			if !hasAccess {
				// I will test if one of the group account if part of hare access denied.
				p, err := self.getPersistenceStore()
				if err != nil {
					return false, false, err
				}

				// Here I will test if a newer token exist for that user if it's the case
				// I will not refresh that token.
				values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+subject+`"}`, ``)
				if err == nil {
					// Validate group organization
					group := values.(map[string]interface{})
					if group["organizations"] != nil {
						organizations := []interface{}(group["organizations"].(primitive.A))
						if organizations != nil {
							for i := 0; i < len(organizations); i++ {
								organizationId := organizations[i].(map[string]interface{})["$id"].(string)
								hasAccess_, _, _ := self.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
								if hasAccess_ {
									return true, false, nil
								}
							}
						}
					}
				}
			}
		} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
			if allowed.Organizations != nil {
				hasAccess = Utility.Contains(allowed.Organizations, subject)
				if hasAccess {
					return true, false, nil
				}
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			// Here the Subject is an application.
			if allowed.Peers != nil {
				hasAccess = Utility.Contains(allowed.Peers, subject)
				if hasAccess {
					return true, false, nil
				}
			}
		} else if subjectType == rbacpb.SubjectType_APPLICATION {
			// Here the Subject is an application.
			if allowed.Applications != nil {
				hasAccess = Utility.Contains(allowed.Applications, subject)
				if hasAccess {
					return true, false, nil
				}
			}
		}
	}

	if !hasAccess {
		err := errors.New("Access denied for " + subjectStr + " " + subject + "!")
		return false, false, err
	}

	// The permission is set.
	return true, false, nil
}

//* Validate if a account can get access to a given ressource for a given operation (read, write...) That function is recursive. *
func (self *Globule) ValidateAccess(ctx context.Context, rqst *rbacpb.ValidateAccessRqst) (*rbacpb.ValidateAccessRsp, error) {
	hasAccess, accessDenied, err := self.validateAccess(rqst.Subject, rqst.Type, rqst.Permission, rqst.Path)
	if err != nil || !hasAccess || accessDenied {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The permission is set.
	return &rbacpb.ValidateAccessRsp{Result: true}, nil
}

////////////////////////////////////////////////////////////////////////////////
//
////////////////////////////////////////////////////////////////////////////////

//* Return the action resource informations. That function must be called
// before calling ValidateAction. In that way the list of ressource affected
// by the rpc method will be given and resource access validated.
// ex. CopyFile(src, dest) -> src and dest are resource path and must be validated
// for read and write access respectivly.
func (self *Globule) GetActionResourceInfos(ctx context.Context, rqst *rbacpb.GetActionResourceInfosRqst) (*rbacpb.GetActionResourceInfosRsp, error) {
	infos, err := self.getActionResourcesPermissions(rqst.Action)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetActionResourceInfosRsp{Infos: infos}, nil
}

/**
 * Validate an action and also validate it resources
 */
func (self *Globule) validateAction(action string, subject string, subjectType rbacpb.SubjectType, resources []*rbacpb.ResourceInfos) (bool, error) {
	p, err := self.getPersistenceStore()
	if err != nil {
		return false, err
	}

	var values map[string]interface{}

	// Validate the access for a given suject...
	hasAccess := false

	// So first of all I will validate the actions itself...
	if subjectType == rbacpb.SubjectType_APPLICATION {

		values_, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+subject+`"}`, "")
		if err != nil {
			return false, err
		}
		values = values_.(map[string]interface{})
	} else if subjectType == rbacpb.SubjectType_PEER {
		values_, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+subject+`"}`, "")
		if err != nil {
			return false, err
		}
		values = values_.(map[string]interface{})
	} else if subjectType == rbacpb.SubjectType_ROLE {
		values_, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+subject+`"}`, "")
		if err != nil {
			return false, err
		}
		values = values_.(map[string]interface{})
	} else if subjectType == rbacpb.SubjectType_ACCOUNT {
		values_, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"_id":"`+subject+`"}`, "")
		if err != nil {
			return false, err
		}
		values = values_.(map[string]interface{})
		// call the rpc method.
		if values["roles"].(primitive.A) != nil {
			roles := []interface{}(values["roles"].(primitive.A))
			if roles != nil {
				for i := 0; i < len(roles); i++ {
					roleId := roles[i].(map[string]interface{})["$id"].(string)
					hasAccess_, _ := self.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
					if hasAccess_ {
						hasAccess = hasAccess_
						break
					}
				}
			}
		}
	}

	if !hasAccess {
		if values["actions"] != nil {
			actions := []interface{}(values["actions"].(primitive.A))
			for i := 0; i < len(actions); i++ {
				if actions[i].(string) == action {
					hasAccess = true
				}
			}
		}
	}

	if !hasAccess {
		err := errors.New("Access denied for " + subject + " to call method " + action)
		return false, err
	} else if subjectType == rbacpb.SubjectType_ROLE {
		return true, nil
	}

	log.Println("Access allow for " + subject + " to call method " + action)

	// Here I will validate the access for a given subject...

	log.Println("validate ressource access for ", subject)
	// Now I will validate the resource access.
	// infos
	permissions_, _ := self.getActionResourcesPermissions(action)
	if len(resources) > 0 {
		if permissions_ == nil {
			err := errors.New("No resources path are given for validations!")
			return false, err
		}
		for i := 0; i < len(resources); i++ {
			if len(resources[i].Path) > 0 { // Here if the path is empty i will simply not validate it.
				hasAccess, accessDenied, _ := self.validateAccess(subject, subjectType, resources[i].Permission, resources[i].Path)
				if hasAccess == false || accessDenied == true {
					err := errors.New("Subject " + subject + " can call the method '" + action + "' but has not the permission to " + resources[i].Permission + " resource '" + resources[i].Path + "'")
					return false, err
				} else if hasAccess == true {
					return true, nil
				}
			}
		}
	}

	return true, nil
}

//* Validate the actions...
func (self *Globule) ValidateAction(ctx context.Context, rqst *rbacpb.ValidateActionRqst) (*rbacpb.ValidateActionRsp, error) {

	// If the address is local I will give the permission.
	log.Println("validate action ", rqst.Action, rqst.Subject, rqst.Type, rqst.Infos)

	hasAccess, err := self.validateAction(rqst.Action, rqst.Subject, rqst.Type, rqst.Infos)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateActionRsp{
		Result: hasAccess,
	}, nil
}
