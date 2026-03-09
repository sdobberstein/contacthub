package sqlite_test

import (
	"context"
	"testing"

	"github.com/sdobberstein/contacthub/internal/store"
)

func TestPropertyStore_SetAndGet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	p := &store.Property{
		Resource:  "/dav/addressbooks/alice/personal/alice.vcf",
		Namespace: "http://example.com/ns/",
		Name:      "color",
		Value:     "blue",
	}
	if err := db.SetProperty(ctx, p); err != nil {
		t.Fatalf("SetProperty: %v", err)
	}

	got, err := db.GetProperty(ctx, p.Resource, p.Namespace, p.Name)
	if err != nil {
		t.Fatalf("GetProperty: %v", err)
	}
	if got.Value != "blue" {
		t.Errorf("value: want blue, got %q", got.Value)
	}
}

func TestPropertyStore_SetOverwritesExisting(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	p := &store.Property{Resource: "/r", Namespace: "http://ns/", Name: "x", Value: "v1"}
	_ = db.SetProperty(ctx, p)

	p.Value = "v2"
	if err := db.SetProperty(ctx, p); err != nil {
		t.Fatalf("SetProperty overwrite: %v", err)
	}

	got, _ := db.GetProperty(ctx, p.Resource, p.Namespace, p.Name)
	if got.Value != "v2" {
		t.Errorf("value after overwrite: want v2, got %q", got.Value)
	}
}

func TestPropertyStore_GetNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := db.GetProperty(ctx, "/r", "http://ns/", "nope"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPropertyStore_ListProperties(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	resource := "/dav/addressbooks/alice/personal/alice.vcf"
	_ = db.SetProperty(ctx, &store.Property{Resource: resource, Namespace: "http://ns/", Name: "a", Value: "1"})
	_ = db.SetProperty(ctx, &store.Property{Resource: resource, Namespace: "http://ns/", Name: "b", Value: "2"})
	// Different resource — should not appear.
	_ = db.SetProperty(ctx, &store.Property{Resource: "/other", Namespace: "http://ns/", Name: "c", Value: "3"})

	props, err := db.ListProperties(ctx, resource)
	if err != nil {
		t.Fatalf("ListProperties: %v", err)
	}
	if len(props) != 2 {
		t.Errorf("want 2 properties, got %d", len(props))
	}
}

func TestPropertyStore_DeleteProperty(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	p := &store.Property{Resource: "/r", Namespace: "http://ns/", Name: "x", Value: "v"}
	_ = db.SetProperty(ctx, p)
	if err := db.DeleteProperty(ctx, p.Resource, p.Namespace, p.Name); err != nil {
		t.Fatalf("DeleteProperty: %v", err)
	}
	if _, err := db.GetProperty(ctx, p.Resource, p.Namespace, p.Name); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPropertyStore_DeleteResourceProperties(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	resource := "/r"
	_ = db.SetProperty(ctx, &store.Property{Resource: resource, Namespace: "http://ns/", Name: "a", Value: "1"})
	_ = db.SetProperty(ctx, &store.Property{Resource: resource, Namespace: "http://ns/", Name: "b", Value: "2"})

	if err := db.DeleteResourceProperties(ctx, resource); err != nil {
		t.Fatalf("DeleteResourceProperties: %v", err)
	}

	props, _ := db.ListProperties(ctx, resource)
	if len(props) != 0 {
		t.Errorf("want 0 properties after bulk delete, got %d", len(props))
	}
}
