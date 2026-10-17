## ADDED Requirements

### Requirement: Describe callable portfolio routes
The service SHALL publish an OpenAPI contract matching the currently implemented trade and summary routes, including request fields, response bodies and observed status codes. Proposed capabilities SHALL NOT be represented as callable operations before implementation.

#### Scenario: Contract matches current server
- **WHEN** a consumer loads `api/openapi.yaml`
- **THEN** it finds `POST /v1/portfolios/{id}/trades` and `GET /v1/portfolios/{id}/summary`
- **AND** the contract documents their current 201/400 and 200/400/500 status behaviors
- **AND** it does not claim planned routes are implemented.
