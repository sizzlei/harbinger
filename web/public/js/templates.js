// web/public/js/templates.js

// ----------------------------------------------------
// Helper function to Validate JSON
// ----------------------------------------------------
function validateJson(textareaId) {
    const textarea = document.getElementById(textareaId);
    if (!textarea) return true; // (textarea가 없으면 유효성 검사 통과)

    try {
        JSON.parse(textarea.value);
        textarea.classList.remove('is-invalid');
        return true; // (성공)
    } catch (e) {
        textarea.classList.add('is-invalid');
        return false; // (실패)
    }
}

// ----------------------------------------------------
// Helper function to Format JSON
// ----------------------------------------------------
function formatJson(textareaId) {
    const textarea = document.getElementById(textareaId);
    if (!textarea) return;

    try {
        const parsed = JSON.parse(textarea.value);
        // 2칸 들여쓰기 적용
        textarea.value = JSON.stringify(parsed, null, 2); 
        textarea.classList.remove('is-invalid');
    } catch (e) {
        alert("유효하지 않은 JSON 형식입니다. 포매팅할 수 없습니다.");
        textarea.classList.add('is-invalid');
    }
}

// ----------------------------------------------------
// Helper function to setup Table Search
// ----------------------------------------------------
function setupTableSearch(inputId, tableBodyId, textClass) {
    const searchInput = document.getElementById(inputId);
    const tableBody = document.getElementById(tableBodyId);

    if (!searchInput || !tableBody) return;

    searchInput.addEventListener('keyup', function() {
        const searchTerm = this.value.toLowerCase();
        const rows = tableBody.getElementsByTagName('tr');

        for (let i = 0; i < rows.length; i++) {
            const row = rows[i];
            let textElement = row.querySelector(textClass);
            
            if (!textElement) {
                const tds = row.getElementsByTagName('td');
                if (tds.length > 1) {
                    textElement = tds[1];
                }
            }
            
            if (textElement) {
                const textContent = (textElement.textContent || textElement.innerText).toLowerCase();
                
                if (textContent.indexOf(searchTerm) > -1) {
                    row.style.display = "";
                } else {
                    row.style.display = "none";
                }
            }
        }
    });
}

// ----------------------------------------------------
// (신규) Helper function to Validate Form on Submit
// ----------------------------------------------------
function setupFormValidation(formId, textareaId) {
    const form = document.getElementById(formId);
    if (!form) return;

    form.addEventListener('submit', function(event) {
        // 폼 전송(submit) 시 JSON 유효성 검사
        const isValid = validateJson(textareaId);
        
        if (!isValid) {
            // 유효하지 않으면, 폼 전송을 차단(prevent)
            event.preventDefault();
            alert("JSON 형식이 올바르지 않습니다. 수정 후 다시 시도해 주세요.");
        }
        // (유효하면 폼은 정상적으로 서버에 전송됨)
    });
}


// ----------------------------------------------------
// Initialization (DOM 로드 후 실행)
// ----------------------------------------------------
document.addEventListener('DOMContentLoaded', (event) => {
    // 1. 목록 검색 초기화 (templates.html용)
    setupTableSearch('templateSearchInput', 'templateListBody', '.template-name');

    // 2. JSON 포매터 버튼 (생성 모달)
    const createBtn = document.getElementById('formatJsonBtnCreate');
    if (createBtn) {
        createBtn.addEventListener('click', (e) => {
            e.preventDefault();
            formatJson('template_contents_modal');
        });
    }

    // 3. JSON 포매터 버튼 (수정 페이지)
    const editBtn = document.getElementById('formatJsonBtnEdit');
    if (editBtn) {
        editBtn.addEventListener('click', (e) => {
            e.preventDefault();
            formatJson('template_contents'); 
        });
    }
    
    // 4. (신규) 폼 전송 시 유효성 검사 (생성 모달)
    setupFormValidation('templateCreateForm', 'template_contents_modal');
    
    // 5. (신규) 폼 전송 시 유효성 검사 (수정 페이지)
    // (templates_edit.html의 <form> 태그에 ID가 없으므로 ID를 추가해야 합니다)
    // (우선 ID가 'templateEditForm'이라고 가정)
    setupFormValidation('templateEditForm', 'template_contents');

    
    // (선택 사항) 수정 페이지 로드 시 JSON 포매팅 자동 실행
    if(editBtn) {
        formatJson('template_contents');
    }
});