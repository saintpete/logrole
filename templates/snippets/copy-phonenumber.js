{{- define "copy-phonenumber" }}
<script type="text/javascript">
  var evHandler = function(clipboardElem) {
    return function() {
      var pnCopy = clipboardElem.parentNode.querySelector('.copy-target');
      if (pnCopy === null) {
        return;
      }
      pnCopy.select();
      try {
        result = document.execCommand('copy');
        if (result === false) {
          throw new Error("Could not copy value: " + pnCopy.value);
        }
      } catch (e) {
        console.error(e);
        alert("Couldn't copy text, sorry. Here it is: " + pnCopy.value);
      }
      console.log("Copied "+ pnCopy.value + " to the clipboard");
      pnCopy.blur();
    };
  };

  var clipboards = document.querySelectorAll('.clipboard');
  for (var i = 0; i < clipboards.length; i++) {
    var clipboard = clipboards[i];
    clipboard.addEventListener('click', evHandler(clipboard));
  }
</script>
{{- end }}
