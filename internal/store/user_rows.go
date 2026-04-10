package store

func ApolloUserFromCreateUserRow(row CreateUserRow) ApolloUser {
	return ApolloUser{
		ID:              row.ID,
		StudentID:       row.StudentID,
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		Role:            row.Role,
		Preferences:     row.Preferences,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		EmailVerifiedAt: row.EmailVerifiedAt,
	}
}

func ApolloUserFromGetUserByEmailRow(row GetUserByEmailRow) ApolloUser {
	return ApolloUser{
		ID:              row.ID,
		StudentID:       row.StudentID,
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		Role:            row.Role,
		Preferences:     row.Preferences,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		EmailVerifiedAt: row.EmailVerifiedAt,
	}
}

func ApolloUserFromGetUserByIDRow(row GetUserByIDRow) ApolloUser {
	return ApolloUser{
		ID:              row.ID,
		StudentID:       row.StudentID,
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		Role:            row.Role,
		Preferences:     row.Preferences,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		EmailVerifiedAt: row.EmailVerifiedAt,
	}
}

func ApolloUserFromGetUserByStudentIDRow(row GetUserByStudentIDRow) ApolloUser {
	return ApolloUser{
		ID:              row.ID,
		StudentID:       row.StudentID,
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		Role:            row.Role,
		Preferences:     row.Preferences,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		EmailVerifiedAt: row.EmailVerifiedAt,
	}
}

func ApolloUserFromMarkUserEmailVerifiedRow(row MarkUserEmailVerifiedRow) ApolloUser {
	return ApolloUser{
		ID:              row.ID,
		StudentID:       row.StudentID,
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		Role:            row.Role,
		Preferences:     row.Preferences,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		EmailVerifiedAt: row.EmailVerifiedAt,
	}
}

func ApolloUserFromUpdateUserPreferencesRow(row UpdateUserPreferencesRow) ApolloUser {
	return ApolloUser{
		ID:              row.ID,
		StudentID:       row.StudentID,
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		Role:            row.Role,
		Preferences:     row.Preferences,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		EmailVerifiedAt: row.EmailVerifiedAt,
	}
}
