// web/public/js/channels.js

// ----------------------------------------------------
// Helper function to handle search logic
// ----------------------------------------------------
function setupTableSearch(inputId, tableBodyId, textClass) {
    const searchInput = document.getElementById(inputId);
    const tableBody = document.getElementById(tableBodyId);

    if (!searchInput || !tableBody) {
        // console.warn(`Search setup failed: Input (${inputId}) or Table (${tableBodyId}) not found.`);
        return;
    }

    searchInput.addEventListener('keyup', function() {
        const searchTerm = this.value.toLowerCase(); // 1. 검색어(소문자)
        
        // tableBody 내부의 모든 <tr> 요소를 가져옵니다.
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
                    row.style.display = ""; // 포함되면 <tr>을 보여줍니다.
                } else {
                    row.style.display = "none"; // 포함되지 않으면 <tr>을 숨깁니다.
                }
            }
        }
    });
}

// ----------------------------------------------------
// (신규) Helper function to setup Edit Modals
// ----------------------------------------------------
function setupEditModal(modalId, formId, actionUrlPrefix, idAttribute, inputMap) {
    const editModal = document.getElementById(modalId);
    if (!editModal) return;

    // 모달이 열리기 *직전*에 발생하는 이벤트를 감지합니다.
    editModal.addEventListener('show.bs.modal', function (event) {
        // 1. 이벤트를 일으킨 '수정' 버튼을 찾습니다.
        const button = event.relatedTarget;

        // 2. 버튼의 'data-*' 속성에서 값(ID)을 추출합니다.
        const id = button.getAttribute(idAttribute); // 예: 'data-group-id'

        // 3. 모달 내부의 폼(Form)을 찾습니다.
        const form = editModal.querySelector(formId);
        
        // 4. 폼의 'action' URL을 동적으로 설정합니다.
        // (예: /channels/groups/edit/5)
        form.action = `${actionUrlPrefix}${id}`;
        
        // 5. 폼 내부의 입력(input) 필드에 값을 채워넣습니다.
        for (const inputId in inputMap) {
            const dataKey = inputMap[inputId]; // 예: 'data-group-name'
            const value = button.getAttribute(dataKey);
            const inputField = editModal.querySelector(inputId); // 예: '#edit_group_name_modal'
            if (inputField) {
                inputField.value = value;
            }
        }
    });
}


// ----------------------------------------------------
// Initialization (DOM이 준비된 후 실행)
// ----------------------------------------------------
document.addEventListener('DOMContentLoaded', (event) => {
    // 1. 채널 그룹 검색 초기화 (좌측)
    setupTableSearch('groupSearchInput', 'groupMappingTableBody', '.group-name');

    // 2. 상세 채널 검색 초기화 (우측)
    setupTableSearch('channelSearchInput', 'channelMappingTableBody', '.channel-name');

    // 3. (신규) '채널 그룹 수정' 모달 초기화
    setupEditModal(
        'editGroupModal',                   // 모달 ID
        '#groupEditForm',                   // 폼 ID
        '/channels/groups/edit/',           // 폼 Action URL (Prefix)
        'data-group-id',                    // ID를 가져올 data 속성
        {                                   // (Input ID : data 속성) 맵
            '#edit_group_name_modal': 'data-group-name',
            '#edit_group_desc_modal': 'data-group-desc'
        }
    );

    // 4. (신규) '상세 채널 수정' 모달 초기화
    setupEditModal(
        'editDetailModal',                  // 모달 ID
        '#detailEditForm',                  // 폼 ID
        '/channels/details/edit/',          // 폼 Action URL (Prefix)
        'data-detail-id',                   // ID를 가져올 data 속성
        {                                   // (Input ID : data 속성) 맵
            '#edit_detail_name_modal': 'data-detail-name',
            '#edit_detail_id_modal': 'data-detail-slackid'
        }
    );
});