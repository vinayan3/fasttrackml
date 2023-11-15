//go:build integration

package flows

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/request"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/response"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/common"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/tests/integration/golang/helpers"
)

type ExperimentFlowTestSuite struct {
	helpers.BaseTestSuite
}

// TestExperimentFlowTestSuite tests the full `experiments` flow connected to namespace functionality.
// Flow contains next endpoints:
// - `POST /experiments/create`
// - `POST /experiments/update`
// - `POST /experiments/delete`
// - `POST /experiments/restore`
// - `GET /experiments/search`
// - `GET /experiments/get`
// - `GET /experiments/list`
// - `GET /experiments/get-by-name`
// - `POST /experiments/set-experiment-tag`
func TestExperimentFlowTestSuite(t *testing.T) {
	suite.Run(t, new(ExperimentFlowTestSuite))
}

func (s *ExperimentFlowTestSuite) TearDownTest() {
	require.Nil(s.T(), s.NamespaceFixtures.UnloadFixtures())
}

func (s *ExperimentFlowTestSuite) Test_Ok() {
	tests := []struct {
		name           string
		setup          func() (*models.Namespace, *models.Namespace)
		namespace1Code string
		namespace2Code string
	}{
		{
			name: "TestCustomNamespaces",
			setup: func() (*models.Namespace, *models.Namespace) {
				return &models.Namespace{
						Code:                "namespace-1",
						DefaultExperimentID: common.GetPointer(int32(0)),
					}, &models.Namespace{
						Code:                "namespace-2",
						DefaultExperimentID: common.GetPointer(int32(0)),
					}
			},
			namespace1Code: "namespace-1",
			namespace2Code: "namespace-2",
		},
		{
			name: "TestObviousDefaultAndCustomNamespaces",
			setup: func() (*models.Namespace, *models.Namespace) {
				return &models.Namespace{
						Code:                "default",
						DefaultExperimentID: common.GetPointer(int32(0)),
					}, &models.Namespace{
						Code:                "namespace-1",
						DefaultExperimentID: common.GetPointer(int32(0)),
					}
			},
			namespace1Code: "default",
			namespace2Code: "namespace-1",
		},
		{
			name: "TestImplicitDefaultAndCustomNamespaces",
			setup: func() (*models.Namespace, *models.Namespace) {
				return &models.Namespace{
						Code:                "default",
						DefaultExperimentID: common.GetPointer(int32(0)),
					}, &models.Namespace{
						Code:                "namespace-1",
						DefaultExperimentID: common.GetPointer(int32(0)),
					}
			},
			namespace1Code: "",
			namespace2Code: "namespace-1",
		},
	}

	// delete everything before the test, because when service starts under the hood we create
	// default namespace and experiment, so it could lead to the problems with actual tests.
	require.Nil(s.T(), s.NamespaceFixtures.UnloadFixtures())
	for _, tt := range tests {
		s.T().Run(tt.name, func(T *testing.T) {
			defer require.Nil(s.T(), s.NamespaceFixtures.UnloadFixtures())

			// 1. setup data under the test.
			namespace1, namespace2 := tt.setup()
			_, err := s.NamespaceFixtures.CreateNamespace(context.Background(), namespace1)
			require.Nil(s.T(), err)
			_, err = s.NamespaceFixtures.CreateNamespace(context.Background(), namespace2)
			require.Nil(s.T(), err)

			// 2. run actual flow test over the test data.
			s.testExperimentFlow(tt.namespace1Code, tt.namespace2Code)
		})
	}
}

func (s *ExperimentFlowTestSuite) testExperimentFlow(namespace1Code, namespace2Code string) {
	// test `POST /experiments/create` endpoint.
	// create experiments in scope of different namespaces.
	experiment1ID := s.createExperiment(namespace1Code, &request.CreateExperimentRequest{
		Name:             "ExperimentName1",
		ArtifactLocation: "/artifact/location",
	})
	experiment2ID := s.createExperiment(namespace2Code, &request.CreateExperimentRequest{
		Name:             "ExperimentName2",
		ArtifactLocation: "/artifact/location",
	})

	// test `GET /experiments/get` endpoint.
	// check that experiments were created in scope of difference namespaces.
	experiment1 := s.getExperimentByIDAndCompare(
		namespace1Code,
		experiment1ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:               experiment1ID,
				Name:             "ExperimentName1",
				Tags:             []response.ExperimentTagPartialResponse{},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)
	experiment2 := s.getExperimentByIDAndCompare(
		namespace2Code,
		experiment2ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:               experiment2ID,
				Name:             "ExperimentName2",
				Tags:             []response.ExperimentTagPartialResponse{},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)

	// test `GET /experiments/get` endpoint.
	// check that there is no intersection between experiments, so when we request
	// experiment 1 in scope of namespace 2 and experiment 2 in scope of namespace 1 API will throw an error.
	resp := api.ErrorResponse{}
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodGet,
		).WithNamespace(
			namespace2Code,
		).WithQuery(
			request.GetExperimentRequest{
				ID: experiment1ID,
			},
		).WithResponse(
			&resp,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsGetRoute),
		),
	)
	assert.Equal(
		s.T(),
		fmt.Sprintf(
			"RESOURCE_DOES_NOT_EXIST: unable to find experiment '%s': error getting experiment by id: %s: record not found",
			experiment1ID,
			experiment1ID,
		),
		resp.Error(),
	)
	assert.Equal(s.T(), api.ErrorCodeResourceDoesNotExist, string(resp.ErrorCode))

	resp = api.ErrorResponse{}
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodGet,
		).WithNamespace(
			namespace1Code,
		).WithQuery(
			request.GetExperimentRequest{
				ID: experiment2ID,
			},
		).WithResponse(
			&resp,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsGetRoute),
		),
	)
	assert.Equal(
		s.T(),
		fmt.Sprintf(
			"RESOURCE_DOES_NOT_EXIST: unable to find experiment '%s': error getting experiment by id: %s: record not found",
			experiment2ID,
			experiment2ID,
		),
		resp.Error(),
	)
	assert.Equal(s.T(), api.ErrorCodeResourceDoesNotExist, string(resp.ErrorCode))

	// test `GET /experiments/get-by-name` endpoint.
	// check that experiments were created in scope of difference namespaces.
	s.getExperimentByNameAndCompare(
		namespace1Code,
		experiment1.Experiment.Name,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:               experiment1ID,
				Name:             "ExperimentName1",
				Tags:             []response.ExperimentTagPartialResponse{},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)
	s.getExperimentByNameAndCompare(
		namespace2Code,
		experiment2.Experiment.Name,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:               experiment2ID,
				Name:             "ExperimentName2",
				Tags:             []response.ExperimentTagPartialResponse{},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)

	// test `GET /experiments/search` endpoint.
	s.searchExperimentAndCompare(namespace1Code, []*response.ExperimentPartialResponse{
		experiment1.Experiment,
	})
	s.searchExperimentAndCompare(namespace2Code, []*response.ExperimentPartialResponse{
		experiment2.Experiment,
	})

	// 6. test `POST /experiments/update` endpoint.
	s.updateExperiment(namespace1Code, &request.UpdateExperimentRequest{
		ID:   experiment1.Experiment.ID,
		Name: "UpdatedExperiment1",
	})
	s.updateExperiment(namespace2Code, &request.UpdateExperimentRequest{
		ID:   experiment2.Experiment.ID,
		Name: "UpdatedExperiment2",
	})

	// check that experiments were updated.
	s.getExperimentByIDAndCompare(
		namespace1Code,
		experiment1ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:               experiment1ID,
				Name:             "UpdatedExperiment1",
				Tags:             []response.ExperimentTagPartialResponse{},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)
	s.getExperimentByIDAndCompare(
		namespace2Code,
		experiment2ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:               experiment2ID,
				Name:             "UpdatedExperiment2",
				Tags:             []response.ExperimentTagPartialResponse{},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)

	// test `POST /experiments/set-experiment-tag` endpoint.
	s.setExperimentTag(namespace1Code, &request.SetExperimentTagRequest{
		ID:    experiment1ID,
		Key:   "KeyTag1",
		Value: "ValueTag1",
	})
	s.setExperimentTag(namespace2Code, &request.SetExperimentTagRequest{
		ID:    experiment2ID,
		Key:   "KeyTag2",
		Value: "ValueTag2",
	})

	// check that experiments tags were updated.
	s.getExperimentByIDAndCompare(
		namespace1Code,
		experiment1ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:   experiment1ID,
				Name: "UpdatedExperiment1",
				Tags: []response.ExperimentTagPartialResponse{
					{
						Key:   "KeyTag1",
						Value: "ValueTag1",
					},
				},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)
	s.getExperimentByIDAndCompare(
		namespace2Code,
		experiment2ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:   experiment2ID,
				Name: "UpdatedExperiment2",
				Tags: []response.ExperimentTagPartialResponse{
					{
						Key:   "KeyTag2",
						Value: "ValueTag2",
					},
				},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)

	// test `POST /experiments/delete` endpoint.
	s.deleteExperiment(namespace1Code, experiment1.Experiment.ID)
	s.deleteExperiment(namespace2Code, experiment2.Experiment.ID)

	// check that experiment lifecycle has been updated.
	s.getExperimentByIDAndCompare(
		namespace1Code,
		experiment1ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:   experiment1ID,
				Name: "UpdatedExperiment1",
				Tags: []response.ExperimentTagPartialResponse{
					{
						Key:   "KeyTag1",
						Value: "ValueTag1",
					},
				},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageDeleted),
			},
		},
	)
	s.getExperimentByIDAndCompare(
		namespace2Code,
		experiment2ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:   experiment2ID,
				Name: "UpdatedExperiment2",
				Tags: []response.ExperimentTagPartialResponse{
					{
						Key:   "KeyTag2",
						Value: "ValueTag2",
					},
				},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageDeleted),
			},
		},
	)

	// test `POST /experiments/restore` endpoint.
	s.restoreExperiment(namespace1Code, experiment1ID)
	s.restoreExperiment(namespace2Code, experiment2ID)

	// check that experiment lifecycle has been updated.
	s.getExperimentByIDAndCompare(
		namespace1Code,
		experiment1ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:   experiment1ID,
				Name: "UpdatedExperiment1",
				Tags: []response.ExperimentTagPartialResponse{
					{
						Key:   "KeyTag1",
						Value: "ValueTag1",
					},
				},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)
	s.getExperimentByIDAndCompare(
		namespace2Code,
		experiment2ID,
		&response.GetExperimentResponse{
			Experiment: &response.ExperimentPartialResponse{
				ID:   experiment2ID,
				Name: "UpdatedExperiment2",
				Tags: []response.ExperimentTagPartialResponse{
					{
						Key:   "KeyTag2",
						Value: "ValueTag2",
					},
				},
				ArtifactLocation: "/artifact/location",
				LifecycleStage:   string(models.LifecycleStageActive),
			},
		},
	)
}

func (s *ExperimentFlowTestSuite) createExperiment(
	namespace string, req *request.CreateExperimentRequest,
) string {
	resp := response.CreateExperimentResponse{}
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodPost,
		).WithNamespace(
			namespace,
		).WithRequest(
			req,
		).WithResponse(
			&resp,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsCreateRoute),
		),
	)

	return resp.ID
}

func (s *ExperimentFlowTestSuite) updateExperiment(namespace string, req *request.UpdateExperimentRequest) {
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodPost,
		).WithNamespace(
			namespace,
		).WithRequest(
			req,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsUpdateRoute),
		),
	)
}

func (s *ExperimentFlowTestSuite) searchExperimentAndCompare(
	namespace string, expectedExperiments []*response.ExperimentPartialResponse,
) {
	searchResp := response.SearchExperimentsResponse{}
	require.Nil(
		s.T(),
		s.MlflowClient.WithQuery(
			request.SearchExperimentsRequest{},
		).WithNamespace(
			namespace,
		).WithResponse(
			&searchResp,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsSearchRoute),
		),
	)
	assert.Equal(s.T(), len(expectedExperiments), len(searchResp.Experiments))
	assert.Equal(s.T(), "", searchResp.NextPageToken)
	assert.Equal(s.T(), expectedExperiments, searchResp.Experiments)
}

func (s *ExperimentFlowTestSuite) getExperimentByIDAndCompare(
	namespace string, experimentID string, expectedResponse *response.GetExperimentResponse,
) *response.GetExperimentResponse {
	resp := response.GetExperimentResponse{}
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodGet,
		).WithNamespace(
			namespace,
		).WithQuery(
			request.GetExperimentRequest{
				ID: experimentID,
			},
		).WithResponse(
			&resp,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsGetRoute),
		),
	)
	assert.Equal(s.T(), expectedResponse.Experiment.ID, resp.Experiment.ID)
	assert.Equal(s.T(), expectedResponse.Experiment.Name, resp.Experiment.Name)
	assert.Equal(s.T(), expectedResponse.Experiment.Tags, resp.Experiment.Tags)
	assert.Equal(s.T(), expectedResponse.Experiment.LifecycleStage, resp.Experiment.LifecycleStage)
	assert.Equal(s.T(), expectedResponse.Experiment.ArtifactLocation, resp.Experiment.ArtifactLocation)
	return &resp
}

func (s *ExperimentFlowTestSuite) getExperimentByNameAndCompare(
	namespace string, name string, expectedResponse *response.GetExperimentResponse,
) {
	resp := response.GetExperimentResponse{}
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodGet,
		).WithNamespace(
			namespace,
		).WithQuery(
			request.GetExperimentRequest{
				Name: name,
			},
		).WithResponse(
			&resp,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsGetByNameRoute),
		),
	)
	assert.Equal(s.T(), expectedResponse.Experiment.ID, resp.Experiment.ID)
	assert.Equal(s.T(), expectedResponse.Experiment.Name, resp.Experiment.Name)
	assert.Equal(s.T(), expectedResponse.Experiment.Tags, resp.Experiment.Tags)
	assert.Equal(s.T(), expectedResponse.Experiment.LifecycleStage, resp.Experiment.LifecycleStage)
	assert.Equal(s.T(), expectedResponse.Experiment.ArtifactLocation, resp.Experiment.ArtifactLocation)
}

func (s *ExperimentFlowTestSuite) deleteExperiment(namespace, experiment1ID string) {
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodPost,
		).WithNamespace(
			namespace,
		).WithRequest(
			request.DeleteExperimentRequest{
				ID: experiment1ID,
			},
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsDeleteRoute),
		),
	)
}

func (s *ExperimentFlowTestSuite) restoreExperiment(namespace, experiment1ID string) {
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodPost,
		).WithNamespace(
			namespace,
		).WithRequest(
			request.RestoreExperimentRequest{
				ID: experiment1ID,
			},
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsRestoreRoute),
		),
	)
}

func (s *ExperimentFlowTestSuite) setExperimentTag(namespace string, req *request.SetExperimentTagRequest) {
	require.Nil(
		s.T(),
		s.MlflowClient.WithMethod(
			http.MethodPost,
		).WithNamespace(
			namespace,
		).WithRequest(
			req,
		).DoRequest(
			fmt.Sprintf("%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsSetExperimentTag),
		),
	)
}
