package project

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, p *Project) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]Project, error) {
	var items []Project
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetByID(ctx context.Context, id, userID string) (*Project, error) {
	var p Project
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) Update(ctx context.Context, p *Project) error {
	return r.db.WithContext(ctx).Model(&Project{}).Where("id = ? AND user_id = ?", p.ID, p.UserID).Updates(map[string]interface{}{
		"name":        p.Name,
		"description": p.Description,
	}).Error
}

func (r *Repository) Delete(ctx context.Context, id, userID string) error {
	return r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&Project{}).Error
}
