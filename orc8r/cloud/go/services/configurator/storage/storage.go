/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 * All rights reserved.
 *
 * This source code is licensed under the BSD-style license found in the
 * LICENSE file in the root directory of this source tree.
 */

package storage

import (
	"context"
	"fmt"

	"magma/orc8r/cloud/go/storage"

	"github.com/thoas/go-funk"
)

// ConfiguratorStorageFactory creates ConfiguratorStorage implementations bound
// to transactions.
type ConfiguratorStorageFactory interface {
	// InitializeServiceStorage should be called on service start to initialize
	// the tables that configurator storage implementations will depend on.
	InitializeServiceStorage() error

	// StartTransaction returns a ConfiguratorStorage implementation bound to
	// a transaction. Transaction options can be optionally provided.
	StartTransaction(ctx context.Context, opts *TxOptions) (ConfiguratorStorage, error)
}

// TxOptions specifies options for transactions started by
// ConfiguratorStorageFactory
type TxOptions struct {
	ReadOnly bool
}

// ConfiguratorStorage is the interface for the configurator service's access
// to its data storage layer. Each ConfiguratorStorage instance is tied to a
// transaction within which all function calls operate.
type ConfiguratorStorage interface {

	// Commit commits the underlying transaction
	Commit() error

	// Rollback rolls back the underlying transaction
	Rollback() error

	// =======================================================================
	// Network Operations
	// =======================================================================

	// LoadNetworks returns a set of networks corresponding to the provided
	// load criteria. Any networks which aren't found are excluded from the
	// returned value.
	LoadNetworks(ids []string, loadCriteria NetworkLoadCriteria) (NetworkLoadResult, error)

	// CreateNetwork creates a new network. The created network is returned.
	CreateNetwork(network Network) (Network, error)

	// UpdateNetworks updates a set of networks.
	// If an error is encountered during the operation, the function will
	// continue processing the rest of the input. Errors are collected and
	// returned in FailedOperations at the conclusion of the function.
	UpdateNetworks(updates []NetworkUpdateCriteria) (FailedOperations, error)

	// =======================================================================
	// Entity Operations
	// =======================================================================

	// LoadEntities returns a set of entities corresponding to the provided
	// load criteria. Any entities which aren't found are excluded from the
	// returned value.
	LoadEntities(networkID string, filter EntityLoadFilter, loadCriteria EntityLoadCriteria) (EntityLoadResult, error)

	// CreateEntity creates a new entity. The created entity is returned
	// with system-generated fields filled in.
	CreateEntity(networkID string, entity NetworkEntity) (NetworkEntity, error)

	// UpdateEntity updates an entity.
	// The updates to the specified entity will be returned as a NetworkEntity
	// object. Apart from identity fields, only fields which were updated will
	// be filled out, with system-generated IDs included.
	UpdateEntity(networkID string, update EntityUpdateCriteria) (NetworkEntity, error)

	// =======================================================================
	// Graph Operations
	// =======================================================================

	// LoadGraphForEntity returns the full DAG which contains the requested
	// entity. The load criteria fields on associations are ignored, and the
	// returned entities will always have both association fields filled out.
	LoadGraphForEntity(networkID string, entityID storage.TypeAndKey, loadCriteria EntityLoadCriteria) (EntityGraph, error)
}

// A network represents a tenant. Networks can be configured in a hierarchical
// manner - network-level configurations are assumed to apply across multiple
// entities within the network.
type Network struct {
	ID string

	Name        string
	Description string

	// Configs maps between a type value and a serialized representation of the
	// configuration value. The type value will point to the Serde
	// implementation which can deserialize the associated value.
	Configs map[string][]byte

	Version uint64
}

// InternalNetworkID is the ID of the network under which all non-tenant
// entities are organized under.
const InternalNetworkID = "network_magma_internal"

const internalNetworkName = "Internal Magma Network"
const internalNetworkDescription = "Internal network to hold non-network entities"

// NetworkLoadCriteria specifies how much of a network to load
type NetworkLoadCriteria struct {
	// Set LoadMetadata to true to load metadata fields (name, description)
	LoadMetadata bool

	LoadConfigs bool
}

// FullNetworkLoadCriteria is a utility variable to specify a full network load
var FullNetworkLoadCriteria = NetworkLoadCriteria{LoadMetadata: true, LoadConfigs: true}

type NetworkLoadResult struct {
	Networks           []Network
	NetworkIDsNotFound []string
}

type FailedOperations map[string]error

// NetworkUpdateCriteria specifies how to update a network
type NetworkUpdateCriteria struct {
	// ID of the network to update
	ID string

	// Set DeleteNetwork to true to delete the network
	DeleteNetwork bool

	// Set NewName or NewDescription to nil to indicate that no update is
	// desired. To clear the value of name or description, set these fields to
	// a pointer to an empty string.
	NewName        *string
	NewDescription *string

	// New config values to add or existing ones to update
	ConfigsToAddOrUpdate map[string][]byte

	// Config values to delete
	ConfigsToDelete []string
}

// NetworkEntity is the storage representation of a logical component of a
// network. Networks are partitioned into DAGs of entities.
type NetworkEntity struct {
	// (Type, Key) forms a unique identifier for the network entity within its
	// network.
	Type string
	Key  string

	Name        string
	Description string

	// PhysicalID will be non-empty if the entity corresponds to a physical
	// asset.
	PhysicalID string

	// Serialized view of the entity's configuration. The value of the Type
	// field will determine the Serde implementation for this value.
	Config []byte

	// GraphID is a mostly-internal field to designate the DAG that this
	// network entity belongs to. This field is system-generated and will be
	// ignored if set during entity creation.
	GraphID string

	// Associations are the directed edges originating from this entity.
	Associations []storage.TypeAndKey

	// ParentAssociations are the directed edges ending at this entity.
	// This is a read-only field and will be ignored if set during entity
	// creation.
	ParentAssociations []storage.TypeAndKey

	// Permissions defines the access control for this entity.
	Permissions []ACL

	Version uint64
}

func (ent NetworkEntity) GetTypeAndKey() storage.TypeAndKey {
	return storage.TypeAndKey{Type: ent.Type, Key: ent.Key}
}

func (ent NetworkEntity) GetGraphEdges() []GraphEdge {
	myTk := ent.GetTypeAndKey()
	existingAssocs := map[storage.TypeAndKey]struct{}{}
	ret := make([]GraphEdge, 0, len(ent.Associations))
	for _, assoc := range ent.Associations {
		if _, exists := existingAssocs[assoc]; exists {
			continue
		}
		ret = append(ret, GraphEdge{From: myTk, To: assoc})
		existingAssocs[assoc] = struct{}{}
	}
	return ret
}

// ACL (Access Control List) defines a specific permission for an entity on
// access to other entities.
type ACL struct {
	// A unique system-generated identifier for this ACL.
	ID string

	// An ACL can apply to one or more networks.
	Scope ACLScope

	Permission ACLPermission

	// An ACL can define access permissions to a specific type of entity, or
	// all entities.
	Type ACLType

	// An ACL can optionally define access permissions to specific entity IDs
	// If empty, the ACL will apply to all entities of the specified type.
	IDFilter []string

	Version uint64
}

// ACLScope is a oneof to define the scope of an ACL (specific networks or all
// networks in the system).
type ACLScope struct {
	NetworkIDs []string
	Wildcard   ACLWildcard
}

var WildcardACLScope = ACLScope{Wildcard: WildcardAll}

func ACLScopeOf(networkIDs []string) ACLScope {
	return ACLScope{NetworkIDs: networkIDs}
}

// ACLType is a oneof to define the scope of the permissions of an ACL (apply
// to access on a specific type or all types within the scope).
type ACLType struct {
	EntityType string
	Wildcard   ACLWildcard
}

var WildcardACLType = ACLType{Wildcard: WildcardAll}

func ACLTypeOf(t string) ACLType {
	return ACLType{EntityType: t}
}

type ACLPermission int32

const (
	NoPermissions ACLPermission = iota
	ReadPermission
	WritePermission
	OwnerPermission
)

type ACLWildcard int32

const (
	NoWildcard ACLWildcard = iota
	WildcardAll
)

// EntityLoadFilter specifies which entities to load from storage
type EntityLoadFilter struct {
	// If TypeFilter is provided, the query will return all entities matching
	// the given type.
	TypeFilter *string

	// If KeyFilter is provided, the query will return all entities matching the
	// given ID.
	KeyFilter *string

	// If IDs is provided, the query will return all entities matching the
	// provided TypeAndKeys. TypeFilter and KeyFilter are ignored if IDs is
	// provided.
	IDs []storage.TypeAndKey

	// Unexported for internal use
	graphID *string
}

// IsLoadAllEntities return true if the EntityLoadFilter is specifying to load
// all entities in a network, false if there are any filter conditions.
func (elf EntityLoadFilter) IsLoadAllEntities() bool {
	return elf.TypeFilter == nil && elf.KeyFilter == nil && elf.graphID == nil && funk.IsEmpty(elf.IDs)
}

// EntityLoadCriteria specifies how much of an entity to load
type EntityLoadCriteria struct {
	// Set LoadMetadata to true to load the metadata fields (name, description)
	LoadMetadata bool

	LoadConfig bool

	LoadAssocsToThis   bool
	LoadAssocsFromThis bool

	LoadPermissions bool
}

// FullEntityLoadCriteria is an EntityLoadCriteria which loads everything
var FullEntityLoadCriteria = EntityLoadCriteria{
	LoadMetadata:       true,
	LoadConfig:         true,
	LoadAssocsToThis:   true,
	LoadAssocsFromThis: true,
	LoadPermissions:    true,
}

// EntityLoadResult encapsulates the result of a LoadEntities call
type EntityLoadResult struct {
	// Loaded entities
	Entities []NetworkEntity
	// Entities which were not found
	EntitiesNotFound []storage.TypeAndKey
}

// EntityUpdateCriteria specifies a patch operation on a network entity.
type EntityUpdateCriteria struct {
	// (Type, Key) of the entity to update
	Type string
	Key  string

	// Set DeleteEntity to true to mark the entity for deletion
	DeleteEntity bool

	NewName        *string
	NewDescription *string

	NewPhysicalID *string

	// A nil value here indicates no update.
	NewConfig *[]byte

	AssociationsToAdd    []storage.TypeAndKey
	AssociationsToDelete []storage.TypeAndKey

	// New ACLs to add. ACL IDs are ignored and generated by the system.
	PermissionsToCreate []ACL

	// ACLs to change. The ACL IDs are important here - make sure to provide
	// the same ID returned from loading the entity.
	PermissionsToUpdate []ACL

	// ACL IDs to delete
	PermissionsToDelete []string
}

func (euc EntityUpdateCriteria) GetTypeAndKey() storage.TypeAndKey {
	return storage.TypeAndKey{Type: euc.Type, Key: euc.Key}
}

// EntityGraph represents a DAG of associated network entities.
type EntityGraph struct {
	// All nodes in the graph
	Entities []NetworkEntity

	// All nodes in the graph which don't have any edges terminating at them.
	RootEntities []storage.TypeAndKey

	// All edges in the graph.
	Edges []GraphEdge
}

// GraphEdge represents a directed edge within a graph
type GraphEdge struct {
	To, From storage.TypeAndKey
}

func (edge GraphEdge) String() string {
	return fmt.Sprintf("%s, %s", edge.From, edge.To)
}
