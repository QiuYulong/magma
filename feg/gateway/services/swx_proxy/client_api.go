/*
Copyright (c) Facebook, Inc. and its affiliates.
All rights reserved.

This source code is licensed under the BSD-style license found in the
LICENSE file in the root directory of this source tree.
*/

// Package swx_proxy provides a thin client for using swx proxy service.
// This can be used by apps to discover and contact the service, without knowing about
// the RPC implementation.
package swx_proxy

import (
	"errors"
	"fmt"
	"strings"

	"magma/feg/cloud/go/protos"
	"magma/feg/gateway/registry"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Wrapper for GRPC Client to extend it with Cleanup
// functionality
type swxProxyClient struct {
	protos.SwxProxyClient
	cc *grpc.ClientConn
}

func (cl *swxProxyClient) Cleanup() {
	if cl != nil && cl.cc != nil {
		cl.cc.Close()
	}
}

// getSwxProxyClient is a utility function to get a RPC connection to the
// Swx Proxy service
func getSwxProxyClient() (*swxProxyClient, error) {
	conn, err := registry.GetConnection(registry.SWX_PROXY)
	if err != nil {
		errMsg := fmt.Sprintf("Swx Proxy client initialization error: %s", err)
		glog.Error(errMsg)
		return nil, errors.New(errMsg)
	}
	return &swxProxyClient{
		protos.NewSwxProxyClient(conn),
		conn,
	}, err
}

func getRemoteSwxProxyClient() (*swxProxyClient, error) {
	conn, err := registry.NewCloudRegistry().GetCloudConnection(strings.ToLower(registry.SWX_PROXY))
	if err != nil {
		errMsg := fmt.Sprintf("Remote Swx Proxy client initialization error: %s", err)
		glog.Error(errMsg)
		return nil, errors.New(errMsg)
	}
	return &swxProxyClient{
		protos.NewSwxProxyClient(conn),
		conn,
	}, err
}

// Authenticate sends MAR (code 303) over diameter connection,
// waits (blocks) for MAA & returns its RPC representation
func Authenticate(req *protos.AuthenticationRequest) (*protos.AuthenticationAnswer, error) {
	err := verifyAuthenticationRequest(req)
	if err != nil {
		errMsg := fmt.Errorf("Invalid AuthenticationRequest provided: %s", err)
		return nil, errors.New(errMsg.Error())
	}
	cli, err := getSwxProxyClient()
	if err != nil {
		return nil, err
	}
	defer cli.Cleanup()
	return cli.Authenticate(context.Background(), req)
}

// Register sends SAR (Code 301) over diameter connection with ServerAssignmentType
// set to REGISTRATION, waits (blocks) for SAA & returns its RPC representation
func Register(req *protos.RegistrationRequest) (*protos.RegistrationAnswer, error) {
	err := verifyRegistrationRequest(req)
	if err != nil {
		errMsg := fmt.Errorf("Invalid RegistrationRequest provided: %s", err)
		return nil, errors.New(errMsg.Error())
	}
	cli, err := getSwxProxyClient()
	if err != nil {
		return nil, err
	}
	defer cli.Cleanup()
	return cli.Register(context.Background(), req)
}

// Deregister sends SAR (Code 301) over diameter connection with ServerAssignmentType
// set to USER_DEREGISTRATION, waits (blocks) for SAA & returns its RPC representation
func Deregister(req *protos.RegistrationRequest) (*protos.RegistrationAnswer, error) {
	err := verifyRegistrationRequest(req)
	if err != nil {
		errMsg := fmt.Errorf("Invalid RegistrationRequest provided: %s", err)
		return nil, errors.New(errMsg.Error())
	}
	cli, err := getSwxProxyClient()
	if err != nil {
		return nil, err
	}
	defer cli.Cleanup()
	return cli.Deregister(context.Background(), req)
}

// AuthenticateRemote sends MAR (code 303) to a remote swx_proxy service,
// waits (blocks) for MAA & returns its RPC representation
func AuthenticateRemote(req *protos.AuthenticationRequest) (*protos.AuthenticationAnswer, error) {
	err := verifyAuthenticationRequest(req)
	if err != nil {
		errMsg := fmt.Errorf("Invalid AuthenticationRequest provided: %s", err)
		return nil, errors.New(errMsg.Error())
	}
	cli, err := getRemoteSwxProxyClient()
	if err != nil {
		return nil, err
	}
	defer cli.Cleanup()
	return cli.Authenticate(context.Background(), req)
}

// RegisterRemote sends SAR (Code 301) with ServerAssignmentType to REGISTRATION
// to a remote swx_proxy service, waits (blocks) for SAA & returns its RPC representation
func RegisterRemote(req *protos.RegistrationRequest) (*protos.RegistrationAnswer, error) {
	err := verifyRegistrationRequest(req)
	if err != nil {
		errMsg := fmt.Errorf("Invalid RegistrationRequest provided: %s", err)
		return nil, errors.New(errMsg.Error())
	}
	cli, err := getRemoteSwxProxyClient()
	if err != nil {
		return nil, err
	}
	defer cli.Cleanup()
	return cli.Register(context.Background(), req)
}

func verifyAuthenticationRequest(req *protos.AuthenticationRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	return verifyUsername(req.GetUserName())
}

func verifyRegistrationRequest(req *protos.RegistrationRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	return verifyUsername(req.GetUserName())
}

func verifyUsername(username string) error {
	if len(username) == 0 {
		return fmt.Errorf("no username provided")
	} else if len(username) > 16 {
		return fmt.Errorf("username is too long (must be 16 digits or less)")
	}
	return nil
}
