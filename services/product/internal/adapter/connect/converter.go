package connect

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	productv1 "github.com/daisuke8000/example-ec-platform/gen/product/v1"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

func toProtoProduct(p *domain.Product) *productv1.Product {
	if p == nil {
		return nil
	}
	pb := &productv1.Product{
		Id:          p.ID.String(),
		Name:        p.Name,
		Description: stringOrEmpty(p.Description),
		Status:      toProtoProductStatus(p.Status),
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
	if p.CategoryID != nil {
		pb.CategoryId = p.CategoryID.String()
	}
	return pb
}

func toProtoProductWithSKUs(p *domain.ProductWithSKUs) *productv1.Product {
	if p == nil {
		return nil
	}
	pb := toProtoProduct(p.Product)
	for _, sku := range p.SKUs {
		pb.Skus = append(pb.Skus, toProtoSKU(sku))
	}
	return pb
}

func toProtoProductStatus(s domain.ProductStatus) productv1.ProductStatus {
	switch s {
	case domain.ProductStatusDraft:
		return productv1.ProductStatus_PRODUCT_STATUS_DRAFT
	case domain.ProductStatusPublished:
		return productv1.ProductStatus_PRODUCT_STATUS_PUBLISHED
	case domain.ProductStatusHidden:
		return productv1.ProductStatus_PRODUCT_STATUS_HIDDEN
	default:
		return productv1.ProductStatus_PRODUCT_STATUS_UNSPECIFIED
	}
}

func toProtoSKU(s *domain.SKU) *productv1.SKU {
	if s == nil {
		return nil
	}
	return &productv1.SKU{
		Id:         s.ID.String(),
		ProductId:  s.ProductID.String(),
		SkuCode:    s.SKUCode,
		Price:      toProtoMoney(s.Price),
		Attributes: s.Attributes,
		CreatedAt:  timestamppb.New(s.CreatedAt),
		UpdatedAt:  timestamppb.New(s.UpdatedAt),
	}
}

func toProtoSKUWithInventory(s *domain.SKUWithInventory) *productv1.SKU {
	if s == nil {
		return nil
	}
	pb := toProtoSKU(s.SKU)
	if s.Inventory != nil {
		pb.Inventory = toProtoInventory(s.Inventory)
	}
	return pb
}

func toProtoMoney(m domain.Money) *productv1.Money {
	return &productv1.Money{
		Amount:       m.Amount,
		CurrencyCode: m.Currency,
	}
}

func toProtoCategory(c *domain.Category) *productv1.Category {
	if c == nil {
		return nil
	}
	pb := &productv1.Category{
		Id:        c.ID.String(),
		Name:      c.Name,
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
	if c.ParentID != nil {
		parentID := c.ParentID.String()
		pb.ParentId = &parentID
	}
	return pb
}

func toProtoInventory(i *domain.Inventory) *productv1.Inventory {
	if i == nil {
		return nil
	}
	return &productv1.Inventory{
		SkuId:     i.SKUID.String(),
		Quantity:  i.Quantity,
		Reserved:  i.Reserved,
		Available: i.Available(),
		Version:   i.Version,
	}
}

func toProtoReservation(r *domain.Reservation) *productv1.Reservation {
	if r == nil {
		return nil
	}
	pb := &productv1.Reservation{
		Id:        r.ID.String(),
		Status:    toProtoReservationStatus(r.Status),
		CreatedAt: timestamppb.New(r.CreatedAt),
		ExpiresAt: timestamppb.New(r.ExpiresAt),
	}

	if r.Status == domain.ReservationStatusPending {
		remaining := time.Until(r.ExpiresAt).Seconds()
		if remaining < 0 {
			remaining = 0
		}
		pb.RemainingTtlSeconds = int64(remaining)
	}

	for _, item := range r.Items {
		pb.Items = append(pb.Items, &productv1.ReservationItem{
			SkuId:    item.SKUID.String(),
			Quantity: item.Quantity,
		})
	}
	return pb
}

func toProtoReservationStatus(s domain.ReservationStatus) productv1.ReservationStatus {
	switch s {
	case domain.ReservationStatusPending:
		return productv1.ReservationStatus_RESERVATION_STATUS_PENDING
	case domain.ReservationStatusConfirmed:
		return productv1.ReservationStatus_RESERVATION_STATUS_CONFIRMED
	case domain.ReservationStatusReleased:
		return productv1.ReservationStatus_RESERVATION_STATUS_RELEASED
	case domain.ReservationStatusExpired:
		return productv1.ReservationStatus_RESERVATION_STATUS_EXPIRED
	default:
		return productv1.ReservationStatus_RESERVATION_STATUS_UNSPECIFIED
	}
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
