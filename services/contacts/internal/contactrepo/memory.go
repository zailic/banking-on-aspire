package contactrepo

import (
	"context"
	"sort"
	"strings"
	"sync"

	contactsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/contacts/v1"
	"google.golang.org/protobuf/proto"
)

type MemoryRepository struct {
	mu       sync.RWMutex
	contacts map[string]*contactsv1.Contact
}

func NewMemory() *MemoryRepository {
	return &MemoryRepository{contacts: make(map[string]*contactsv1.Contact)}
}

func (r *MemoryRepository) List(_ context.Context, parent string) ([]*contactsv1.Contact, error) {
	prefix := parent + "/contacts/"
	r.mu.RLock()
	defer r.mu.RUnlock()
	contacts := make([]*contactsv1.Contact, 0)
	for name, contact := range r.contacts {
		if strings.HasPrefix(name, prefix) {
			contacts = append(contacts, clone(contact))
		}
	}
	sort.Slice(
		contacts,
		func(i, j int) bool { return contacts[i].GetName() < contacts[j].GetName() },
	)
	return contacts, nil
}

func (r *MemoryRepository) Get(_ context.Context, name string) (*contactsv1.Contact, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	contact, exists := r.contacts[name]
	if !exists {
		return nil, ErrNotFound
	}
	return clone(contact), nil
}

func (r *MemoryRepository) Create(_ context.Context, contact *contactsv1.Contact) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.contacts[contact.GetName()]; exists {
		return ErrAlreadyExists
	}
	r.contacts[contact.GetName()] = clone(contact)
	return nil
}

func (r *MemoryRepository) Update(
	_ context.Context,
	contact *contactsv1.Contact,
	expectedEtag string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.contacts[contact.GetName()]
	if !exists {
		return ErrNotFound
	}
	if expectedEtag != "" && expectedEtag != current.GetEtag() {
		return ErrConflict
	}
	r.contacts[contact.GetName()] = clone(contact)
	return nil
}

func (r *MemoryRepository) Delete(_ context.Context, name, expectedEtag string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.contacts[name]
	if !exists {
		return ErrNotFound
	}
	if expectedEtag != "" && expectedEtag != current.GetEtag() {
		return ErrConflict
	}
	delete(r.contacts, name)
	return nil
}

func clone(contact *contactsv1.Contact) *contactsv1.Contact {
	return proto.Clone(contact).(*contactsv1.Contact)
}
