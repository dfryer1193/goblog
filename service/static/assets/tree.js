// goblog's only script, and an optional one: the tree pane's folders are
// <details> and work without it. It remembers which folders the reader
// folded, so they stay folded on the next page, and keeps the pane and the
// phone drawer's copy of the tree in step. A folder holding the page being
// shown is never folded on load.
(function () {
  "use strict";
  var key = "goblog.folded";
  var folded = {};
  try {
    folded = JSON.parse(window.localStorage.getItem(key)) || {};
  } catch (e) {
    folded = {};
  }

  var folds = document.querySelectorAll(".tree details.fold[data-fold]");
  // shown is each fold's last known state. A toggle event that doesn't
  // change it isn't the reader's: browsers fire one for a <details> parsed
  // with open, and for the folds this script sets.
  var shown = new Map();

  function setOpen(fold, open) {
    shown.set(fold, open);
    fold.open = open;
  }

  // holdsCurrent reports whether the page being shown is under fold, not
  // the fold's own heading.
  function holdsCurrent(fold) {
    var current = fold.querySelector("[aria-current]");
    return current !== null && current.closest("summary") === null;
  }

  folds.forEach(function (fold) {
    var name = fold.dataset.fold;
    setOpen(fold, fold.open && !(folded[name] && !holdsCurrent(fold)));
    fold.addEventListener("toggle", function () {
      if (fold.open === shown.get(fold)) {
        return;
      }
      shown.set(fold, fold.open);
      if (fold.open) {
        delete folded[name];
      } else {
        folded[name] = true;
      }
      folds.forEach(function (other) {
        if (other !== fold && other.dataset.fold === name) {
          setOpen(other, fold.open);
        }
      });
      try {
        window.localStorage.setItem(key, JSON.stringify(folded));
      } catch (e) {
        // Storage is off or full: folds just won't carry over.
      }
    });
  });
})();
