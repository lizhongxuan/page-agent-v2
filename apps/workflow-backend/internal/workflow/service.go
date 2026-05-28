package workflow

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (service *Service) Create(ctx context.Context, recipe WorkflowRecipe) (WorkflowRecipe, error) {
	if recipe.Version == 0 {
		recipe.Version = 1
	}
	if err := ValidateRecipe(recipe); err != nil {
		return WorkflowRecipe{}, err
	}
	return service.repo.Save(ctx, recipe)
}

func (service *Service) Get(ctx context.Context, id string) (WorkflowRecipe, error) {
	return service.repo.Get(ctx, id)
}

func (service *Service) AddVersion(ctx context.Context, id string, recipe WorkflowRecipe) (WorkflowRecipe, error) {
	current, err := service.repo.Get(ctx, id)
	if err != nil {
		return WorkflowRecipe{}, err
	}
	recipe.ID = id
	recipe.ProjectID = current.ProjectID
	recipe.Status = current.Status
	recipe.Version = current.Version + 1
	if err := ValidateRecipe(recipe); err != nil {
		return WorkflowRecipe{}, err
	}
	return service.repo.Save(ctx, recipe)
}

func (service *Service) Promote(ctx context.Context, id string) error {
	recipe, err := service.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	recipe.Status = WorkflowStatusActive
	_, err = service.repo.Save(ctx, recipe)
	return err
}

func (service *Service) Disable(ctx context.Context, id string) error {
	recipe, err := service.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	recipe.Status = WorkflowStatusDisabled
	_, err = service.repo.Save(ctx, recipe)
	return err
}

func (service *Service) Search(ctx context.Context, request SearchRequest) ([]SearchResult, error) {
	recipes, err := service.repo.List(ctx, request.ProjectID)
	if err != nil {
		return nil, err
	}
	return rankWorkflows(recipes, request), nil
}
