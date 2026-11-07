package db

import (
	"campusclaw/backend/internal/config"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSuppressedMaterialSeedIntegration(t *testing.T) {
	if os.Getenv("TUTOR_INTEGRATION") != "1" {
		t.Skip("requires compose DB")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = Migrate(d); err != nil {
		t.Fatal(err)
	}
	tag := fmt.Sprint(time.Now().UnixNano())
	res, err := d.Exec(`INSERT INTO classes(name) VALUES(?)`, "seed-suppression-"+tag)
	if err != nil {
		t.Fatal(err)
	}
	cls, _ := res.LastInsertId()
	defer func() {
		d.Exec(`DELETE FROM knowledge_entries WHERE class_id=?`, cls)
		d.Exec(`DELETE FROM materials WHERE class_id=?`, cls)
		d.Exec(`DELETE FROM material_seed_suppressions WHERE class_id=?`, cls)
		d.Exec(`DELETE FROM classes WHERE id=?`, cls)
	}()
	title := "种子" + tag
	if err = ensureSeedMaterial(d, cls, 1, title, "原始正文"); err != nil {
		t.Fatal(err)
	}
	d.Exec(`UPDATE materials SET title='教师改名' WHERE class_id=?`, cls)
	d.Exec(`INSERT INTO material_seed_suppressions(class_id,title) VALUES(?,?)`, cls, title)
	if err = ensureSeedMaterial(d, cls, 1, title, "不要复活"); err != nil {
		t.Fatal(err)
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM materials WHERE class_id=?`, cls).Scan(&n)
	if n != 1 {
		t.Fatal("renamed seed respawned")
	}
	d.Exec(`DELETE FROM materials WHERE class_id=?`, cls)
	if err = ensureSeedMaterial(d, cls, 1, title, "不要复活"); err != nil {
		t.Fatal(err)
	}
	d.QueryRow(`SELECT COUNT(*) FROM materials WHERE class_id=?`, cls).Scan(&n)
	if n != 0 {
		t.Fatal("deleted seed respawned")
	}
}
