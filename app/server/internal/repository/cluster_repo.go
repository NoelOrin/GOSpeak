package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

// ClusterNodeRepository 负责 cluster_nodes 表访问。
type ClusterNodeRepository struct {
	db *gorm.DB
}

func NewClusterNodeRepository(db *gorm.DB) *ClusterNodeRepository {
	return &ClusterNodeRepository{db: db}
}

func (r *ClusterNodeRepository) Create(node *model.ClusterNode) error {
	return r.db.Create(node).Error
}

func (r *ClusterNodeRepository) Update(node *model.ClusterNode) error {
	return r.db.Save(node).Error
}

func (r *ClusterNodeRepository) GetByUUID(uuid string) (*model.ClusterNode, error) {
	var node model.ClusterNode
	err := r.db.Where("uuid = ?", uuid).First(&node).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *ClusterNodeRepository) GetByName(name string) (*model.ClusterNode, error) {
	var node model.ClusterNode
	err := r.db.Where("name = ?", name).First(&node).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *ClusterNodeRepository) List() ([]model.ClusterNode, error) {
	var nodes []model.ClusterNode
	err := r.db.Order("created_at ASC").Find(&nodes).Error
	return nodes, err
}

// MarkOfflineBefore 将超过超时未上报的活跃节点标记为 offline。
func (r *ClusterNodeRepository) MarkOfflineBefore(before time.Time) error {
	return r.db.Model(&model.ClusterNode{}).
		Where("last_seen_at < ? AND status IN ?", before, []string{
			model.ClusterNodePending,
			model.ClusterNodeReady,
			model.ClusterNodeBusy,
		}).
		Update("status", model.ClusterNodeOffline).Error
}

// ServerAssignmentRepository 负责 server_assignments 表访问。
type ServerAssignmentRepository struct {
	db *gorm.DB
}

func NewServerAssignmentRepository(db *gorm.DB) *ServerAssignmentRepository {
	return &ServerAssignmentRepository{db: db}
}

func (r *ServerAssignmentRepository) Ensure(serverUUID, nodeUUID string) error {
	var existing model.ServerAssignment
	err := r.db.Where("server_uuid = ? AND node_uuid = ?", serverUUID, nodeUUID).First(&existing).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.Create(&model.ServerAssignment{
		ServerUUID: serverUUID,
		NodeUUID:   nodeUUID,
		Status:     model.ServerAssignmentAssigned,
	}).Error
}

func (r *ServerAssignmentRepository) Remove(serverUUID, nodeUUID string) error {
	return r.db.Where("server_uuid = ? AND node_uuid = ?", serverUUID, nodeUUID).Delete(&model.ServerAssignment{}).Error
}

func (r *ServerAssignmentRepository) RemoveAll(serverUUID string) error {
	return r.db.Where("server_uuid = ?", serverUUID).Delete(&model.ServerAssignment{}).Error
}

func (r *ServerAssignmentRepository) ListByServer(serverUUID string) ([]model.ServerAssignment, error) {
	var assignments []model.ServerAssignment
	err := r.db.Where("server_uuid = ?", serverUUID).Order("created_at ASC").Find(&assignments).Error
	return assignments, err
}

func (r *ServerAssignmentRepository) ListByNode(nodeUUID string) ([]model.ServerAssignment, error) {
	var assignments []model.ServerAssignment
	err := r.db.Where("node_uuid = ?", nodeUUID).Order("created_at ASC").Find(&assignments).Error
	return assignments, err
}

func (r *ServerAssignmentRepository) CountByNode(nodeUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.ServerAssignment{}).Where("node_uuid = ?", nodeUUID).Count(&count).Error
	return count, err
}

func (r *ServerAssignmentRepository) UpdateStatus(serverUUID, nodeUUID, status string) error {
	return r.db.Model(&model.ServerAssignment{}).
		Where("server_uuid = ? AND node_uuid = ?", serverUUID, nodeUUID).
		Update("status", status).Error
}
