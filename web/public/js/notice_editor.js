// web/public/js/notice_editor.js

document.addEventListener('DOMContentLoaded', (event) => {
    
    // 1. [공지 생성] 모달용 에디터 (notices.html)
    const createModal = document.getElementById('createNoticeModal');
    if (createModal) {
        let easyMDE_Create = null; // 생성 모달 MDE 인스턴스

        // 모달이 '열릴 때' 이벤트를 감지
        createModal.addEventListener('shown.bs.modal', function () {
            // 이미 초기화되었다면 중복 실행 방지
            if (easyMDE_Create) {
                easyMDE_Create.codemirror.refresh(); // 이미 있다면 새로고침
                return;
            }

            // EasyMDE를 'content_body_modal' textarea에 적용
            easyMDE_Create = new EasyMDE({
                element: document.getElementById('content_body_modal'),
                toolbar: ["bold", "italic", "strikethrough", "|", "heading-1", "heading-2", "heading-3", "|", "code", "quote", "|", "unordered-list", "ordered-list", "|", "link", "table", "|", "preview", "side-by-side", "fullscreen"],
                spellChecker: false,
            });
        });
    }

    // 2. [공지 수정] 페이지용 에디터 (notices_edit.html)
    const editTextArea = document.getElementById('content_body_edit');
    if (editTextArea) {
        // EasyMDE를 'content_body_edit' textarea에 적용
        const easyMDE_Edit = new EasyMDE({
            element: editTextArea,
            toolbar: ["bold", "italic", "strikethrough", "|", "heading-1", "heading-2", "heading-3", "|", "code", "quote", "|", "unordered-list", "ordered-list", "|", "link", "table", "|", "preview", "side-by-side", "fullscreen"],
            spellChecker: false,
        });
    }
});