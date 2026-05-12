// Niyantra Dashboard — Keyboard Shortcuts
import { switchToTab } from '../core/theme';
import { closeModal, openModal, closeDelete } from '../subscriptions';
import { closeBudget } from '../overview/budget';
import { handleSnap } from './snap';
import { toggleCommandPalette } from './palette';


export function initKeyboardShortcuts(): void {
  document.addEventListener('keydown', function(e) {
    // Skip if user is typing in an input/textarea/select
    var tag = document.activeElement?.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
      if (e.key === 'Escape') {
        (document.activeElement as HTMLElement)?.blur();
        closeModal();
        closeDelete();
        closeBudget();
      }
      return;
    }

    // Don't fire shortcuts when modals are open (except Escape)
    var anyModal = !document.getElementById('modal-overlay')!.hidden ||
                   !document.getElementById('delete-overlay')!.hidden ||
                   !document.getElementById('budget-overlay')!.hidden;

    if (e.key === 'Escape') {
      closeModal();
      closeDelete();
      closeBudget();
      return;
    }

    if (anyModal) return;

    switch (e.key) {
      case '1': switchToTab('quotas'); break;
      case '2': switchToTab('subscriptions'); break;
      case '3': switchToTab('overview'); break;
      case '4': switchToTab('settings'); break;
      case 'n': case 'N': openModal(); e.preventDefault(); break;
      case 's': case 'S': handleSnap(); e.preventDefault(); break;
      case '/':
        e.preventDefault();
        switchToTab('subscriptions');
        setTimeout(function() {
          var search = document.getElementById('search-subs');
          if (search) search.focus();
        }, 100);
        break;
    }
  });

  // Ctrl+K / Cmd+K for command palette
  document.addEventListener('keydown', function(e) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      toggleCommandPalette();
    }
  });
}
