package query

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
)

type QueryTestSuite struct {
	db *gorm.DB
	suite.Suite
}

func TestQueryTestSuite(t *testing.T) {
	suite.Run(t, new(QueryTestSuite))
}

func (s *QueryTestSuite) SetupTest() {
	mockedDB, _, err := sqlmock.New()
	require.Nil(s.T(), err)

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:       mockedDB,
		DriverName: "postgres",
	}), &gorm.Config{})
	require.Nil(s.T(), err)
	s.db = db
}

func (s *QueryTestSuite) TestPostgresDialector_Ok() {
	tests := []struct {
		name          string
		query         string
		selectMetrics bool
		expectedSQL   string
		expectedVars  []interface{}
	}{
		{
			name:  "TestRunNameWithoutFunction",
			query: `(run.name == 'run')`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" = $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithInFunction",
			query: `('run' in run.name)`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"%run%", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithNotInFunction",
			query: `('run' not in run.name)`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" NOT LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"%run%", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithStartWithFunction",
			query: `(run.name.startswith('run'))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run%", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithEndWithFunction",
			query: `(run.name.endswith('run'))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"%run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithRegexpMatchFunction",
			query: `(re.match('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" ~ $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"^run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithRegexpSearchFunction",
			query: `(re.search('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" ~ $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithNegatedRegexpMatchFunction",
			query: `not (re.match('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" !~ $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"^run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithNegatedRegexpSearchFunction",
			query: `not (re.search('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" !~ $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestNegativeInteger",
			query: `run.metrics['my_metric'].last < -1`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`WHERE "metrics_0"."value" < $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"my_metric", -1, models.LifecycleStageDeleted},
		},
		{
			name:  "TestNegativeFloat",
			query: `run.metrics['my_metric'].last < -1.0`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`WHERE "metrics_0"."value" < $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"my_metric", -1.0, models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContext",
			query:         `metric.context.key1 == 'value1'`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 = $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"{key1}", "value1", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextNegative",
			query:         `metric.context.key1 != 'value1'`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 <> $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"{key1}", "value1", models.LifecycleStageDeleted},
		},
		{
			name:  "TestMetricContextSliceTuple",
			query: `run.metrics["my_metric", {"key1": "value1"}].last < -1`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`LEFT JOIN contexts contexts_1 ON metrics_0.context_id = contexts_1.id ` +
				`WHERE "contexts_1"."json"#>>$2 = $3 ` +
				`AND ("metrics_0"."value" < $4 AND "runs"."lifecycle_stage" <> $5)`,
			expectedVars: []interface{}{"my_metric", "{key1}", "value1", -1, models.LifecycleStageDeleted},
		},
		{
			name:  "TestMetricContextSliceTupleWithPrefix",
			query: `run.metrics["my_metric", {"$.key1": "value1"}].last < -1`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`LEFT JOIN contexts contexts_1 ON metrics_0.context_id = contexts_1.id ` +
				`WHERE "contexts_1"."json"#>>$2 = $3 ` +
				`AND ("metrics_0"."value" < $4 AND "runs"."lifecycle_stage" <> $5)`,
			expectedVars: []interface{}{"my_metric", "{key1}", "value1", -1, models.LifecycleStageDeleted},
		},
		{
			name:  "TestMetricCompound",
			query: `run.metrics["my_metric"].last < -1 and metric.context.key1 == "value1"`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`WHERE ("metrics_0"."value" < $2 AND "contexts"."json"#>>$3 = $4) ` +
				`AND "runs"."lifecycle_stage" <> $5`,
			expectedVars: []interface{}{"my_metric", -1, "{key1}", "value1", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextArray",
			query:         `metric.context.key1 == [1,2,3]`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 = '[1, 2, 3]' AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"{key1}", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextObject",
			query:         `metric.context.key1 == {"subkey": "val"}`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 = '{"subkey": "val"}' AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"{key1}", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextObject2",
			query:         `metric.context.key1 == {"subkey": "val", "subkey2": "val"}`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 = '{"subkey": "val", "subkey2": "val"}' AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"{key1}", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextIn",
			query:         `"val1" in metric.context.key1`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"{key1}", "%val1%", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextNotIn",
			query:         `"val1" not in metric.context.key1`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 NOT LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"{key1}", "%val1%", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextStartsWith",
			query:         `metric.context.key1.startswith("va")`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"{key1}", "va%", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextEndsWith",
			query:         `metric.context.key1.endswith("va")`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE "contexts"."json"#>>$1 LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"{key1}", "%va", models.LifecycleStageDeleted},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			pq := QueryParser{
				Default: DefaultExpression{
					Contains:   "run.archived",
					Expression: "not run.archived",
				},
				Tables: map[string]string{
					"runs":        "runs",
					"experiments": "Experiment",
					"metrics":     "metrics",
				},
				Dialector: postgres.Dialector{}.Name(),
			}
			parsedQuery, err := pq.Parse(tt.query)
			require.Nil(s.T(), err)
			var tx *gorm.DB
			if tt.selectMetrics {
				tx = parsedQuery.Filter(
					s.db.Session(&gorm.Session{DryRun: true}).Model(models.Metric{}),
				).Select("ID").Find(models.Metric{})
			} else {
				tx = parsedQuery.Filter(
					s.db.Session(&gorm.Session{DryRun: true}).Model(models.Run{}),
				).Select("ID").Find(&models.Run{})
			}

			require.Nil(s.T(), tx.Error)
			assert.Equal(s.T(), tt.expectedSQL, tx.Statement.SQL.String())
			assert.Equal(s.T(), tt.expectedVars, tx.Statement.Vars)
		})
	}
}

func (s *QueryTestSuite) TestSqliteDialector_Ok() {
	tests := []struct {
		name          string
		query         string
		selectMetrics bool
		expectedSQL   string
		expectedVars  []interface{}
	}{
		{
			name:  "TestRunNameWithoutFunction",
			query: `(run.name == 'run')`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" = $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithInFunction",
			query: `('run' in run.name)`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"%run%", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithNotInFunction",
			query: `('run' not in run.name)`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" NOT LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"%run%", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithStartWithFunction",
			query: `(run.name.startswith('run'))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run%", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithEndWithFunction",
			query: `(run.name.endswith('run'))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE "runs"."name" LIKE $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"%run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithRegexpMatchFunction",
			query: `(re.match('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE IFNULL("runs"."name", '') REGEXP $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"^run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithRegexpSearchFunction",
			query: `(re.search('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE IFNULL("runs"."name", '') REGEXP $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithNegatedRegexpMatchFunction",
			query: `not (re.match('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE IFNULL("runs"."name", '') NOT REGEXP $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"^run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestRunNameWithNegatedRegexpSearchFunction",
			query: `not (re.search('run', run.name))`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`WHERE IFNULL("runs"."name", '') NOT REGEXP $1 AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"run", models.LifecycleStageDeleted},
		},
		{
			name:  "TestNegativeInteger",
			query: `run.metrics['my_metric'].last < -1`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`WHERE "metrics_0"."value" < $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"my_metric", -1, models.LifecycleStageDeleted},
		},
		{
			name:  "TestNegativeFloat",
			query: `run.metrics['my_metric'].last < -1.0`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`WHERE "metrics_0"."value" < $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"my_metric", -1.0, models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContext",
			query:         `metric.context.key1 == 'value1'`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +

				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 = $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"$.key1", "value1", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextNegative",
			query:         `metric.context.key1 != 'value1'`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 <> $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"$.key1", "value1", models.LifecycleStageDeleted},
		},
		{
			name:  "TestMetricKeySlice",
			query: `run.metrics["key1"].last < -1`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`WHERE "metrics_0"."value" < $2 AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"key1", -1, models.LifecycleStageDeleted},
		},
		{
			name:  "TestMetricContextSliceTuple",
			query: `run.metrics["my_metric", {"key1": "value1"}].last < -1`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`LEFT JOIN contexts contexts_1 ON metrics_0.context_id = contexts_1.id ` +
				`WHERE IFNULL("contexts_1"."json", JSON('{}'))->>$2 = $3 ` +
				`AND ("metrics_0"."value" < $4 AND "runs"."lifecycle_stage" <> $5)`,
			expectedVars: []interface{}{"my_metric", "$.key1", "value1", -1, models.LifecycleStageDeleted},
		},
		{
			name:  "TestMetricContextSliceTupleWithPrefix",
			query: `run.metrics["my_metric", {"$.key1": "value1"}].last < -1`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`LEFT JOIN contexts contexts_1 ON metrics_0.context_id = contexts_1.id ` +
				`WHERE IFNULL("contexts_1"."json", JSON('{}'))->>$2 = $3 ` +
				`AND ("metrics_0"."value" < $4 AND "runs"."lifecycle_stage" <> $5)`,
			expectedVars: []interface{}{"my_metric", "$.key1", "value1", -1, models.LifecycleStageDeleted},
		},
		{
			name:  "TestMetricCompound",
			query: `run.metrics["my_metric"].last < -1 and metric.context.key1 == "value1"`,
			expectedSQL: `SELECT "run_uuid" FROM "runs" ` +
				`LEFT JOIN latest_metrics metrics_0 ON runs.run_uuid = metrics_0.run_uuid AND metrics_0.key = $1 ` +
				`WHERE ("metrics_0"."value" < $2 AND IFNULL("contexts"."json", JSON('{}'))->>$3 = $4) ` +
				`AND "runs"."lifecycle_stage" <> $5`,
			expectedVars: []interface{}{"my_metric", -1, "$.key1", "value1", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextArray",
			query:         `metric.context.key1 == [1,2,3]`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 = '[1,2,3]' AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"$.key1", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextObject",
			query:         `metric.context.key1 == {"subkey": "val"}`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 = '{"subkey":"val"}' ` +
				`AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"$.key1", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextObject2",
			query:         `metric.context.key1 == {"subkey": "val", "subkey2": "val"}`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 = '{"subkey":"val","subkey2":"val"}' ` +
				`AND "runs"."lifecycle_stage" <> $2`,
			expectedVars: []interface{}{"$.key1", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextIn",
			query:         `"val1" in metric.context.key1`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"$.key1", "%val1%", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextNotIn",
			query:         `"val1" not in metric.context.key1`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 NOT LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"$.key1", "%val1%", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextStartsWith",
			query:         `metric.context.key1.startswith("va")`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"$.key1", "va%", models.LifecycleStageDeleted},
		},
		{
			name:          "TestMetricContextEndsWith",
			query:         `metric.context.key1.endswith("va")`,
			selectMetrics: true,
			expectedSQL: `SELECT ID FROM "metrics" ` +
				`WHERE IFNULL("contexts"."json", JSON('{}'))->>$1 LIKE $2 ` +
				`AND "runs"."lifecycle_stage" <> $3`,
			expectedVars: []interface{}{"$.key1", "%va", models.LifecycleStageDeleted},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			pq := QueryParser{
				Default: DefaultExpression{
					Contains:   "run.archived",
					Expression: "not run.archived",
				},
				Tables: map[string]string{
					"runs":        "runs",
					"experiments": "Experiment",
					"metrics":     "metrics",
				},
				Dialector: sqlite.Dialector{}.Name(),
			}
			parsedQuery, err := pq.Parse(tt.query)
			require.Nil(s.T(), err)
			var tx *gorm.DB
			if tt.selectMetrics {
				tx = parsedQuery.Filter(
					s.db.Session(&gorm.Session{DryRun: true}).Model(models.Metric{}),
				).Select("ID").Find(models.Metric{})
			} else {
				tx = parsedQuery.Filter(
					s.db.Session(&gorm.Session{DryRun: true}).Model(models.Run{}),
				).Select("ID").Find(&models.Run{})
			}

			require.Nil(s.T(), tx.Error)
			assert.Equal(s.T(), tt.expectedSQL, tx.Statement.SQL.String())
			assert.Equal(s.T(), tt.expectedVars, tx.Statement.Vars)
		})
	}
}

func (s *QueryTestSuite) Test_Error() {
	tests := []struct {
		name          string
		query         string
		expectedError error
	}{
		{
			name:          "TestMetricContextNested",
			query:         `metric.context.parent.nested == 'value1'`,
			expectedError: SyntaxError{},
		},
		{
			name:          "TestMetricContextSubscriptTupleWrongOrder",
			query:         `run.metrics[{"key1": "value1"}, "my_metric"].last < -1`,
			expectedError: SyntaxError{},
		},
		{
			name:          "TestMetricContextSubscriptTupleDictOnly",
			query:         `run.metrics[{"key1": "value1"}].last < -1`,
			expectedError: SyntaxError{},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			pq := QueryParser{
				Default: DefaultExpression{
					Contains:   "run.archived",
					Expression: "not run.archived",
				},
				Tables: map[string]string{
					"runs":        "runs",
					"experiments": "Experiment",
					"metrics":     "metrics",
				},
				Dialector: sqlite.Dialector{}.Name(),
			}
			parsedQuery, err := pq.Parse(tt.query)
			require.IsType(s.T(), tt.expectedError, err)
			require.Nil(s.T(), parsedQuery)
		})
	}
}
