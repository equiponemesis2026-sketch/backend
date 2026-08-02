package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/nemesis-project/api-nemesis/internal/user/domain"
)

// Errores predefinidos de negocio para evitar fugas de información.
var (
	ErrUserAlreadyExists  = errors.New("a user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidInput       = errors.New("invalid input")
)

var validate = validator.New()

type userUseCase struct {
	userRepo      domain.UserRepository
	contactLinker ContactLinker
	jwtSecret     []byte
	jwtExpiry     time.Duration
}

// ContactLinker permite vincular contactos pendientes al nuevo usuario
// cuando coincide su email/teléfono. Lo implementa el repositorio de contactos.
type ContactLinker interface {
	LinkPendingContacts(ctx context.Context, email string, phone string, userID string) error
}

// PinVerifier permite verificar el PIN ingresado por una víctima contra
// sus hashes de seguridad (real / coerción). Lo implementa el usecase de
// usuarios y lo consumen otros módulos (p.ej. el motor de alertas).
type PinVerifier interface {
	VerifyPin(ctx context.Context, userID string, pin string) (domain.PinMatch, error)
}

// NewUserUseCase inicializa el caso de uso con su repositorio y parámetros JWT.
func NewUserUseCase(repo domain.UserRepository, jwtSecret string, jwtExpiry time.Duration) domain.UserUseCase {
	return &userUseCase{
		userRepo:  repo,
		jwtSecret: []byte(jwtSecret),
		jwtExpiry: jwtExpiry,
	}
}

// SetContactLinker inyecta el linker de contactos pendientes (opcional).
func (u *userUseCase) SetContactLinker(linker ContactLinker) {
	u.contactLinker = linker
}

// SetSecurityPins valida, hashea y persiste los PINs de seguridad del usuario.
func (u *userUseCase) SetSecurityPins(ctx context.Context, userID string, input domain.SecurityPinsInput) error {
	if err := validate.Struct(input); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if input.RealPIN == "" || input.CoercionPIN == "" {
		return ErrInvalidInput
	}

	realHash, err := bcrypt.GenerateFromPassword([]byte(input.RealPIN), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash real pin: %w", err)
	}
	coercionHash, err := bcrypt.GenerateFromPassword([]byte(input.CoercionPIN), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash coercion pin: %w", err)
	}

	if err := u.userRepo.UpdateSecurityPins(ctx, userID, string(realHash), string(coercionHash)); err != nil {
		return fmt.Errorf("failed to persist security pins: %w", err)
	}

	return nil
}

// VerifyPin compara un PIN en texto plano contra los hashes del usuario.
// Devuelve PinNone si no coincide con ninguno o si el usuario no existe.
func (u *userUseCase) VerifyPin(ctx context.Context, userID string, pin string) (domain.PinMatch, error) {
	user, err := u.userRepo.FindByID(ctx, userID)
	if err != nil {
		return domain.PinNone, fmt.Errorf("failed to fetch user: %w", err)
	}
	if user == nil {
		return domain.PinNone, nil
	}

	if user.RealPINHash != "" {
		if bcrypt.CompareHashAndPassword([]byte(user.RealPINHash), []byte(pin)) == nil {
			return domain.PinReal, nil
		}
	}
	if user.CoercionPINHash != "" {
		if bcrypt.CompareHashAndPassword([]byte(user.CoercionPINHash), []byte(pin)) == nil {
			return domain.PinCoercion, nil
		}
	}

	return domain.PinNone, nil
}

// Register maneja la lógica de negocio para la creación y persistencia de un nuevo usuario.
func (u *userUseCase) Register(ctx context.Context, input domain.RegisterInput) (*domain.User, error) {
	// 1. Validar campos (email formato, password min 8, requeridos)
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	// 2. Normalizar email (lowercase + trim)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	// 3. Validar unicidad de email
	existingUser, err := u.userRepo.FindByEmail(ctx, input.Email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// 4. Hashear password con Bcrypt usando costo de producción (12)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 5. Generar ID único con prefijo de dominio 'usr_' y UUIDv4
	userID := fmt.Sprintf("usr_%s", uuid.New().String())

	newUser := &domain.User{
		ID:        userID,
		Name:      input.Name,
		Email:     input.Email,
		Password:  string(hashedPassword),
		Phone:     input.Phone,
		CreatedAt: time.Now().UTC(),
	}

	// 6. Persistir registro en base de datos
	if err := u.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to persist user: %w", err)
	}

	// 7. Backfill: vincular contactos pendientes que coincidan con email/teléfono
	if u.contactLinker != nil {
		if err := u.contactLinker.LinkPendingContacts(ctx, newUser.Email, newUser.Phone, newUser.ID); err != nil {
			return nil, fmt.Errorf("failed to link pending contacts: %w", err)
		}
	}

	// 8. Sanitizar la entidad de salida limpiando credenciales sensibles
	newUser.Password = ""

	return newUser, nil
}

// Login valida la existencia del usuario, verifica contraseñas y genera el token JWT firmado.
func (u *userUseCase) Login(ctx context.Context, input domain.LoginInput) (*domain.TokenResponse, error) {
	// 1. Obtener usuario (email normalizado)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	user, err := u.userRepo.FindByEmail(ctx, input.Email)
	if err != nil || user == nil {
		return nil, ErrInvalidCredentials
	}

	// 2. Verificar contraseña hash
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// 3. Crear Claims para el JWT
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(u.jwtExpiry).Unix(),
		"iat":   time.Now().Unix(),
	}

	// 4. Firmar el Token utilizando HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(u.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign jwt: %w", err)
	}

	// 5. Generar respuesta estructurada compatible con el Frontend
	response := &domain.TokenResponse{
		AccessToken: tokenString,
		TokenType:   "Bearer",
		ExpiresIn:   int64(u.jwtExpiry.Seconds()),
		User: domain.UserBasic{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		},
	}

	return response, nil
}
