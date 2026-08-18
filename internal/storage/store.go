// Package storage fournit une persistance JSON locale, sûre et sans dépendance
// externe.
//
// Choix technique : JSON plutôt que CSV.
// Les données de gestion sont imbriquées (une facture contient N lignes) et
// typées (dates, booléens, entiers). Le CSV, tabulaire et sans types, obligerait
// à aplatir les documents sur plusieurs fichiers reliés à la main, sans garantie
// d'intégrité. Le JSON conserve la structure exacte, se relit sans ambiguïté et
// reste inspectable dans n'importe quel éditeur de texte. L'export CSV reste
// disponible pour Excel (voir services/export.go).
//
// Garanties d'écriture :
//   - écriture atomique : fichier temporaire + fsync + rename, jamais de fichier
//     à moitié écrit même en cas de coupure de courant ;
//   - verrou lecture/écriture par collection (sûr pour les appels concurrents
//     que Wails émet depuis l'interface) ;
//   - cache mémoire : les lectures ne touchent pas le disque.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"comptoir/internal/models"
)

// ErrNotFound est renvoyée lorsqu'aucun enregistrement ne porte l'identifiant
// demandé.
var ErrNotFound = errors.New("enregistrement introuvable")

// ErrDuplicate est renvoyée lors d'une insertion avec un identifiant déjà pris.
var ErrDuplicate = errors.New("identifiant déjà utilisé")

// Collection est un magasin typé, en mémoire, persisté dans un fichier JSON.
type Collection[T models.Entity] struct {
	mu    sync.RWMutex
	path  string
	items []T
}

// OpenCollection charge (ou crée) le fichier JSON d'une collection.
func OpenCollection[T models.Entity](dir, name string) (*Collection[T], error) {
	c := &Collection[T]{
		path:  filepath.Join(dir, name+".json"),
		items: []T{},
	}
	if err := c.load(); err != nil {
		return nil, fmt.Errorf("chargement de %s : %w", name, err)
	}
	return c, nil
}

func (c *Collection[T]) load() error {
	raw, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return c.persist() // crée un fichier `[]` valide dès le départ
	}
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	var items []T
	if err := json.Unmarshal(raw, &items); err != nil {
		// Le fichier existe mais est illisible : on le met de côté plutôt que
		// de l'écraser, pour permettre une récupération manuelle.
		_ = os.Rename(c.path, c.path+".corrupt")
		return fmt.Errorf("fichier illisible, sauvegardé sous %s.corrupt : %w", c.path, err)
	}
	c.items = items
	return nil
}

// persist écrit la collection sur disque de façon atomique.
// L'appelant doit détenir le verrou en écriture.
func (c *Collection[T]) persist() error {
	return WriteJSONAtomic(c.path, c.items)
}

// All renvoie une copie de tous les enregistrements. La copie évite qu'un
// appelant modifie le cache par accident.
func (c *Collection[T]) All() []T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]T, len(c.items))
	copy(out, c.items)
	return out
}

// Count renvoie le nombre d'enregistrements.
func (c *Collection[T]) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Get récupère un enregistrement par identifiant.
func (c *Collection[T]) Get(id string) (T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, it := range c.items {
		if it.GetID() == id {
			return it, nil
		}
	}
	var zero T
	return zero, ErrNotFound
}

// Find renvoie les enregistrements satisfaisant le prédicat.
func (c *Collection[T]) Find(pred func(T) bool) []T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []T
	for _, it := range c.items {
		if pred(it) {
			out = append(out, it)
		}
	}
	return out
}

// FindOne renvoie le premier enregistrement satisfaisant le prédicat.
func (c *Collection[T]) FindOne(pred func(T) bool) (T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, it := range c.items {
		if pred(it) {
			return it, nil
		}
	}
	var zero T
	return zero, ErrNotFound
}

// Insert ajoute un enregistrement et écrit immédiatement sur disque.
func (c *Collection[T]) Insert(item T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, it := range c.items {
		if it.GetID() == item.GetID() {
			return ErrDuplicate
		}
	}
	c.items = append(c.items, item)
	if err := c.persist(); err != nil {
		c.items = c.items[:len(c.items)-1] // annulation en mémoire
		return err
	}
	return nil
}

// Update remplace un enregistrement existant.
func (c *Collection[T]) Update(item T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, it := range c.items {
		if it.GetID() == item.GetID() {
			previous := c.items[i]
			c.items[i] = item
			if err := c.persist(); err != nil {
				c.items[i] = previous
				return err
			}
			return nil
		}
	}
	return ErrNotFound
}

// Delete supprime un enregistrement.
func (c *Collection[T]) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, it := range c.items {
		if it.GetID() == id {
			removed := c.items[i]
			c.items = append(c.items[:i], c.items[i+1:]...)
			if err := c.persist(); err != nil {
				c.items = append(c.items, removed)
				return err
			}
			return nil
		}
	}
	return ErrNotFound
}

// Transaction applique une mutation sur l'ensemble des enregistrements de façon
// atomique : soit tout est écrit, soit rien ne change. Utilisé lorsqu'une
// opération métier touche plusieurs enregistrements d'une même collection
// (émission de facture, annulation, inventaire).
func (c *Collection[T]) Transaction(mutate func(items []T) ([]T, error)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := make([]T, len(c.items))
	copy(snapshot, c.items)

	next, err := mutate(snapshot)
	if err != nil {
		return err
	}
	previous := c.items
	c.items = next
	if err := c.persist(); err != nil {
		c.items = previous
		return err
	}
	return nil
}

// Path renvoie le chemin du fichier de la collection.
func (c *Collection[T]) Path() string { return c.path }

// ---------------------------------------------------------------------------
// Écriture atomique
// ---------------------------------------------------------------------------

// WriteJSONAtomic sérialise v puis l'écrit à l'emplacement path sans jamais
// laisser de fichier partiel : écriture dans un temporaire du même répertoire,
// synchronisation disque, puis renommage (opération atomique sur NTFS comme sur
// APFS/ext4).
func WriteJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("sérialisation : %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op si le renommage a réussi

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o640); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// ReadJSON charge un document JSON unique (utilisé pour settings.json).
func ReadJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
