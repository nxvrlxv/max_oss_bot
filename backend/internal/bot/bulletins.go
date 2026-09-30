package bot

import (
	"context"
	"errors"
	"time"

	"oss-max/internal/storage"
)

// Почему бюллетень не выдан. Текст показывается человеку как есть —
// и в приложении, и в чате с ботом.
var (
	ErrBulletinsClosed = errors.New("Голосование сейчас не идёт — бюллетени не выдаются")
	ErrNotConfirmed    = errors.New("Бюллетень появится, когда инициатор сверит вашу заявку с реестром собственников")
	ErrNobodyLeft      = errors.New("Голоса всех собственников уже учтены — обходить некого")
)

// BulletinStore — откуда берутся бюллетени.
type BulletinStore interface {
	OwnBulletin(ctx context.Context, meetingID int, maxID int64) ([]storage.Bulletin, error)
	BlankBulletins(ctx context.Context, meetingID int) ([]storage.Bulletin, error)
}

// Bulletins выбирает, что прислать по кнопке: собственнику — его бюллетень,
// инициатору — бланки для обхода. Одна проверка на оба пути: кнопку
// в приложении и «Начать» по ссылке в чате с ботом.
func Bulletins(ctx context.Context, store BulletinStore, meeting Meeting, maxID int64, action Action) ([]storage.Bulletin, error) {
	// После срока бланки уже не соберёшь, до публикации — не на что голосовать.
	if !meeting.Inviting(time.Now()) {
		return nil, ErrBulletinsClosed
	}

	if action == ActionBlanks {
		if meeting.InitiatorID != maxID {
			return nil, storage.ErrNotFound
		}
		bulletins, err := store.BlankBulletins(ctx, meeting.ID)
		if err == nil && len(bulletins) == 0 {
			err = ErrNobodyLeft
		}
		return bulletins, err
	}

	bulletins, err := store.OwnBulletin(ctx, meeting.ID, maxID)
	if err == nil && len(bulletins) == 0 {
		err = ErrNotConfirmed
	}
	return bulletins, err
}
