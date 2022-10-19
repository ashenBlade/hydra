package shared

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Case struct {
	Name     string
	SQL      string
	Validate func(t *testing.T, row pgx.Row)
}

var AcceptanceCases = []Case{
	{
		Name: "columnar ext",
		SQL: `
SELECT count(1) FROM pg_available_extensions WHERE name = 'columnar';
			`,
		Validate: func(t *testing.T, row pgx.Row) {
			var count int
			if err := row.Scan(&count); err != nil {
				t.Fatal(err)
			}

			if want, got := 1, count; want != got {
				t.Fatalf("columnar ext should exist")
			}
		},
	},
	{
		Name: "using a columnar table",
		SQL: `
CREATE TABLE my_columnar_table
(
    id INT,
    i1 INT,
    i2 INT8,
    n NUMERIC,
    t TEXT
) USING columnar;
			`,
	},
	{
		Name: "convert between row and columnar",
		SQL: `
		CREATE TABLE my_table(i INT8 DEFAULT '7');
		INSERT INTO my_table VALUES(1);
		-- convert to columnar
		SELECT columnar.alter_table_set_access_method('my_table', 'columnar');
		-- back to row
		-- TODO: reenable this after it's supported
		-- SELECT alter_table_set_access_method('my_table', 'heap');
		`,
	},
	{
		Name: "convert by copying",
		SQL: `
CREATE TABLE table_heap (i INT8);
CREATE TABLE table_columnar (LIKE table_heap) USING columnar;
INSERT INTO table_columnar SELECT * FROM table_heap;
			`,
	},
	{
		Name: "partition",
		SQL: `
CREATE TABLE parent(ts timestamptz, i int, n numeric, s text)
  PARTITION BY RANGE (ts);

-- columnar partition
CREATE TABLE p0 PARTITION OF parent
  FOR VALUES FROM ('2020-01-01') TO ('2020-02-01')
  USING COLUMNAR;
-- columnar partition
CREATE TABLE p1 PARTITION OF parent
  FOR VALUES FROM ('2020-02-01') TO ('2020-03-01')
  USING COLUMNAR;
-- row partition
CREATE TABLE p2 PARTITION OF parent
  FOR VALUES FROM ('2020-03-01') TO ('2020-04-01');

INSERT INTO parent VALUES ('2020-01-15', 10, 100, 'one thousand'); -- columnar
INSERT INTO parent VALUES ('2020-02-15', 20, 200, 'two thousand'); -- columnar
INSERT INTO parent VALUES ('2020-03-15', 30, 300, 'three thousand'); -- row

CREATE INDEX p2_ts_idx ON p2 (ts);
CREATE UNIQUE INDEX p2_i_unique ON p2 (i);
ALTER TABLE p2 ADD UNIQUE (n);
			`,
	},
	{
		Name: "options",
		SQL: `
SELECT alter_columnar_table_set(
    'my_columnar_table',
    compression => 'none',
    stripe_row_limit => 10000);
			`,
	},
}

var BeforeUpgradeCases = []Case{
	{
		Name: "create columnar table",
		SQL: `
CREATE TABLE columnar_table
(
    id UUID,
    i1 INT,
    i2 INT8,
    n NUMERIC,
    t TEXT
) USING columnar;
		`,
	},
	{
		Name: "insert into columnar table",
		SQL: `
INSERT INTO columnar_table (id, i1, i2, n, t)
VALUES ('75372aac-d74a-4e5a-8bf3-43cdaf9011de', 2, 3, 100.1, 'hydra');
		`,
	},
}

var AfterUpgradeCases = []Case{
	{
		Name: "create another columnar table",
		SQL: `
CREATE TABLE columnar_table2
(
    id UUID,
    i1 INT,
    i2 INT8,
    n NUMERIC,
    t TEXT
) USING columnar;
		`,
	},
	{
		Name: "validate columnar data",
		SQL:  "SELECT id, i1, i2, n, t FROM columnar_table LIMIT 1;",
		Validate: func(t *testing.T, row pgx.Row) {
			var result struct {
				ID uuid.UUID
				I1 int
				I2 int
				N  float32
				T  string
			}

			if err := row.Scan(&result.ID, &result.I1, &result.I2, &result.N, &result.T); err != nil {
				t.Fatal(err)
			}

			if result.ID != uuid.MustParse("75372aac-d74a-4e5a-8bf3-43cdaf9011de") {
				t.Errorf("id returned %s after upgrade, expected 75372aac-d74a-4e5a-8bf3-43cdaf9011de", result.ID)
			}

			if result.I1 != 2 {
				t.Errorf("i1 returned %d after upgrade, expected 2", result.I1)
			}

			if result.I2 != 3 {
				t.Errorf("i2 returned %d after upgrade, expected 3", result.I2)
			}

			if result.N != 100.1 {
				t.Errorf("n returned %f after upgrade, expected 100.1", result.N)
			}

			if result.T != "hydra" {
				t.Errorf("t returned %s after upgrade, expected hydra", result.T)
			}
		},
	},
}
