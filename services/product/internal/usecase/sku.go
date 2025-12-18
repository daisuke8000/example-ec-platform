package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type SKUUseCase interface {
	CreateSKU(ctx context.Context, input CreateSKUInput) (*domain.SKU, error)
	GetSKU(ctx context.Context, id uuid.UUID) (*domain.SKU, error)
	GetSKUWithInventory(ctx context.Context, id uuid.UUID) (*domain.SKUWithInventory, error)
	GetSKUsByProductID(ctx context.Context, productID uuid.UUID) ([]*domain.SKU, error)
	UpdateSKU(ctx context.Context, id uuid.UUID, input UpdateSKUInput) (*domain.SKU, error)
	DeleteSKU(ctx context.Context, id uuid.UUID) error
}

type CreateSKUInput struct {
	ProductID       uuid.UUID
	SKUCode         string
	PriceAmount     int64
	PriceCurrency   string
	Attributes      map[string]string
	InitialQuantity int64
}

type UpdateSKUInput struct {
	SKUCode       *string
	PriceAmount   *int64
	PriceCurrency *string
	Attributes    map[string]string
}

type skuUseCase struct {
	skuRepo       domain.SKURepository
	productRepo   domain.ProductRepository
	inventoryRepo domain.InventoryRepository
}

func NewSKUUseCase(skuRepo domain.SKURepository, productRepo domain.ProductRepository, inventoryRepo domain.InventoryRepository) SKUUseCase {
	return &skuUseCase{
		skuRepo:       skuRepo,
		productRepo:   productRepo,
		inventoryRepo: inventoryRepo,
	}
}

func (uc *skuUseCase) CreateSKU(ctx context.Context, input CreateSKUInput) (*domain.SKU, error) {
	if _, err := uc.productRepo.FindByID(ctx, input.ProductID); err != nil {
		return nil, err
	}

	exists, err := uc.skuRepo.ExistsBySKUCode(ctx, input.SKUCode, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrSKUCodeAlreadyExists
	}

	price, err := domain.NewMoney(input.PriceAmount, input.PriceCurrency)
	if err != nil {
		return nil, err
	}

	sku, err := domain.NewSKU(input.ProductID, input.SKUCode, *price, input.Attributes)
	if err != nil {
		return nil, err
	}

	if err := uc.skuRepo.Create(ctx, sku); err != nil {
		return nil, err
	}

	inventory, err := domain.NewInventory(sku.ID, input.InitialQuantity)
	if err != nil {
		return nil, err
	}
	if err := uc.inventoryRepo.Create(ctx, inventory); err != nil {
		return nil, err
	}

	return sku, nil
}

func (uc *skuUseCase) GetSKU(ctx context.Context, id uuid.UUID) (*domain.SKU, error) {
	return uc.skuRepo.FindByID(ctx, id)
}

func (uc *skuUseCase) GetSKUWithInventory(ctx context.Context, id uuid.UUID) (*domain.SKUWithInventory, error) {
	return uc.skuRepo.FindByIDWithInventory(ctx, id)
}

func (uc *skuUseCase) GetSKUsByProductID(ctx context.Context, productID uuid.UUID) ([]*domain.SKU, error) {
	return uc.skuRepo.FindByProductID(ctx, productID)
}

func (uc *skuUseCase) UpdateSKU(ctx context.Context, id uuid.UUID, input UpdateSKUInput) (*domain.SKU, error) {
	sku, err := uc.skuRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	skuCode := sku.SKUCode
	if input.SKUCode != nil {
		skuCode = *input.SKUCode
		if skuCode != sku.SKUCode {
			exists, err := uc.skuRepo.ExistsBySKUCode(ctx, skuCode, &id)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, domain.ErrSKUCodeAlreadyExists
			}
		}
	}

	price := sku.Price
	if input.PriceAmount != nil {
		price.Amount = *input.PriceAmount
	}
	if input.PriceCurrency != nil {
		price.Currency = *input.PriceCurrency
	}

	attributes := sku.Attributes
	if input.Attributes != nil {
		attributes = input.Attributes
	}

	if err := sku.Update(skuCode, price, attributes); err != nil {
		return nil, err
	}

	if err := uc.skuRepo.Update(ctx, sku); err != nil {
		return nil, err
	}
	return sku, nil
}

func (uc *skuUseCase) DeleteSKU(ctx context.Context, id uuid.UUID) error {
	return uc.skuRepo.SoftDelete(ctx, id)
}
