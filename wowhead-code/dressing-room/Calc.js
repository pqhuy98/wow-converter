WH.calc = new function() {
  var e = 2;
  var t = false;
  var i = false;
  var r = false;
  var n = {};
  var a = null;
  var s = {};
  this.init = function(e) {
      if (e.calculator) {
          a = e.calculator;
          if (a.getEmbed)
              this.setEmbed(a.getEmbed());
          if (a.getLocked)
              this.setLocked(a.getLocked());
          if (a.getLockOnLoad)
              this.setLockOnLoad(a.getLockOnLoad());
          if (a.getHashLocked)
              WH.calc.hash.setLocked(a.getHashLocked());
          if (a.getUrlRoot)
              WH.calc.hash.setUrlRoot(a.getUrlRoot());
          if (a.getHashTemplates)
              WH.calc.hash.setTemplates(a.getHashTemplates());
          if (a.getIcons)
              this.setIcons(a.getIcons())
      }
      if (e.embed)
          this.setEmbed(e.embed);
      if (e.locked)
          this.setLocked(e.locked);
      if (e.lockOnLoad)
          this.setLockOnLoad(e.lockOnLoad);
      if (e.hashLocked)
          WH.calc.hash.setLocked(e.hashLocked);
      if (e.urlRoot)
          WH.calc.hash.setUrlRoot(e.urlRoot);
      if (e.hashTemplates)
          WH.calc.hash.setTemplates(e.hashTemplates);
      if (e.icons)
          this.setIcons(e.icons);
      if (e.data)
          this.setData(e.data);
      if (e.postInit) {
          this.flexibleExecuteRequired(e.postInit)
      }
      if (!e.noHashManagement) {
          $(window).bind("hashchange", WH.calc.hash.check.bind(WH.calc.hash)).bind("slashhash", WH.calc.hash.check.bind(WH.calc.hash));
          WH.calc.hash.check()
      }
  }
  ;
  this.setEmbed = function(e) {
      t = e
  }
  ;
  this.setLockOnLoad = function(e) {
      r = e
  }
  ;
  this.setIcon = function(e, t) {
      var i = e.split("/");
      var r = n;
      for (var a = 0; a < i.length; a++) {
          if (!r[i[a]])
              r[i[a]] = {};
          r = r[i[a]]
      }
      r.data = {
          identifier: e,
          html: t
      }
  }
  ;
  this.setIcons = function(e, t) {
      if (t) {
          n = e;
          return
      }
      for (var i in e) {
          this.setIcon(i, e[i])
      }
  }
  ;
  this.setData = function(e) {
      if (a && a.setData) {
          a.setData(e);
          return
      }
      s = e
  }
  ;
  this.getCalculator = function() {
      return a
  }
  ;
  this.getData = function() {
      if (a && a.getData) {
          return a.getData()
      }
      return s
  }
  ;
  this.getVersion = function() {
      return e
  }
  ;
  this.isEmbed = function() {
      return t
  }
  ;
  this.getEmbed = this.isEmbed;
  this.getLocked = function() {
      return i
  }
  ;
  this.isLocked = this.getLocked;
  this.getLockOnLoad = function() {
      return r
  }
  ;
  this.getIcon = function(e, t) {
      var i = e.split("/");
      var r = n;
      for (var a = 0; a < i.length; a++) {
          if (!r[i[a]])
              return;
          r = r[i[a]]
      }
      return this.processIcon(r.data.html, t)
  }
  ;
  this.getIcons = function(e) {
      if (e) {
          return this.processIcons(n, e)
      }
      return n
  }
  ;
  this.processIcon = function(e, t) {
      var i = e;
      if (t && typeof t == "object") {
          for (var r in t) {
              i = i.replace(r, t[r])
          }
      }
      var n = i.match(/%[A-Z]+?%/);
      if (n) {
          WH.calc.externals.log("Did not replace " + (n.length == 1 ? "this icon variable" : "these icon variables") + ": " + n.join(", "), "on", e)
      }
      return i
  }
  ;
  this.processIcons = function(e, t) {
      var i = {};
      for (var r in e) {
          if (e[r].html) {
              i[r] = this.processIcon(e[r], t)
          } else {
              i[r] = this.processIcons(e[r], t)
          }
      }
      return i
  }
  ;
  this.executeRequired = function(e) {
      if (!a) {
          WH.calc.externals.error("Tried to execute calculator functions without a calculator.", arguments);
          return
      }
      if (!a[e]) {
          WH.calc.externals.error("Tried to execute a non-existent calculator function.", arguments);
          return
      }
      return a[e].apply(a, Array.prototype.slice.call(arguments, 1))
  }
  ;
  this.executeOptional = function(e) {
      if (!a) {
          return
      }
      if (!a[e]) {
          return
      }
      return a[e].apply(a, Array.prototype.slice.call(arguments, 1))
  }
  ;
  this.flexibleExecuteRequired = function(e) {
      if (typeof e == "string") {
          return this.executeRequired.apply(this, arguments)
      } else if (typeof e == "function") {
          return e.apply(null, Array.prototype.slice.call(arguments, 1))
      }
  }
}
;
WH.calc.externals = new function() {
  this.DOMAIN = "wowhead";
  this.DEBUG = false;
  function e(e) {
      var t = [];
      for (var i = 0; i < e.length; i++) {
          t[i] = e[i]
      }
      return t
  }
  this.staticUrl = function() {
      return WH.staticUrl
  }
  ;
  this.setDebug = function(e) {
      this.DEBUG = e
  }
  ;
  this.getDebug = function() {
      return this.DEBUG
  }
  ;
  this.debug = function() {
      if (!this.DEBUG) {
          return
      }
      WH.debug.apply(WH, Array.prototype.slice.call(arguments))
  }
  ;
  this.log = WH.log;
  this.error = WH.error;
  this.isInArray = function(e, t) {
      return e.indexOf(t) > -1
  }
  ;
  this.getValueFromObject = function(t, i) {
      if (typeof i == "string" && i.indexOf("/") > -1) {
          i = i.split("/")
      }
      if (this.isInArray(["number", "string"], typeof i)) {
          return t[i]
      }
      if ($.isArray(i)) {
          i = e(i);
          var r = t;
          while (i.length) {
              if (typeof r != "object") {
                  this.debug("Dead end while fetching key from object.", t, i);
                  return
              }
              r = r[i.shift()]
          }
          return r
      }
  }
  ;
  this.setValueOnObject = function(t, i, r) {
      if (typeof i == "string" && i.indexOf("/") > -1) {
          i = i.split("/")
      }
      if (this.isInArray(["number", "string"], typeof i)) {
          t[i] = r;
          return t
      }
      if ($.isArray(i)) {
          i = e(i);
          var n = t, a;
          while (i.length > 1) {
              a = i.shift();
              if (typeof n[a] == "undefined") {
                  n[a] = {}
              }
              if (typeof n[a] != "object") {
                  this.debug("Dead end while setting value on object.", t, i, r);
                  return
              }
              n = n[a]
          }
          n[i.shift()] = r;
          return t
      }
  }
  ;
  this.isMiddleClick = function(e) {
      if (!e.which) {
          return false
      }
      return e.which == 2
  }
  ;
  this.isNotMiddleClick = function(e) {
      if (!e.which) {
          return true
      }
      return e.which != 2
  }
  ;
  this.leadingZeroes = function(e, t) {
      return WH.leadingZeroes(e, t)
  }
  ;
  this.sortObject = function(e, t, i) {
      if (!i)
          i = {};
      var r = {}, n = [], a = 0, s, c;
      for (var l in e) {
          a++;
          if (typeof i.customKey == "function") {
              s = i.customKey(e, t, l, a)
          } else if ($.isArray(t)) {
              c = [];
              for (var o in t) {
                  c.push(e[l][t[o]] || "Ω")
              }
              for (var u = 0, h; h = c[u]; u++) {
                  if (("" + h).match(/^[0-9]+$/)) {
                      c[u] = WH.calc.externals.leadingZeroes(h, 10)
                  }
              }
              s = c.join(" ") + " " + a
          } else if (typeof t != "undefined") {
              if (("" + e[l][t]).match(/^[0-9]+$/)) {
                  s = WH.calc.externals.leadingZeroes(e[l][t], 10)
              } else {
                  s = e[l][t]
              }
              s += " " + a
          } else {
              if (("" + e[l]).match(/^[0-9]+$/)) {
                  s = WH.calc.externals.leadingZeroes(e[l], 10)
              } else {
                  s = e[l]
              }
              s += " " + a
          }
          r[s] = e[l];
          n.push(s)
      }
      n.sort(i.compareFn);
      if (i.reverse)
          n.reverse();
      var f = [];
      for (var l = 0; l < n.length; l++) {
          f.push(r[n[l]])
      }
      return f
  }
  ;
  this.isEqualSimpleObject = function(e, t) {
      return WH.isEqualSimpleObject(e, t)
  }
}
;
WH.calc.action = new function() {
  this.reset = function() {
      WH.calc.executeRequired("reset")
  }
  ;
  this.lock = function() {
      if (WH.calc.isLocked()) {
          WH.calc.unlock()
      } else {
          WH.calc.lock()
      }
  }
  ;
  this.embed = function() {
      prompt("Add this HTML to your web site to embed this build:", "<script>var " + WH.calc.externals.DOMAIN + "_" + WH.calc.executeRequired("getShortName") + ' = { "hash": "' + WH.calc.hash.get() + '" }<\/script><script src="' + WH.calc.externals.staticUrl() + WH.calc.executeRequired("getEmbedFileName") + '"><\/script>')
  }
}
;
WH.calc.hash = new function() {
  var e = false;
  var t = 0;
  var i = 0;
  var r = null;
  var n = null;
  var a = {};
  var s = {
      encoding: "0zMcmVokRsaqbdrfwihuGINALpTjnyxtgevElBCDFHJKOPQSUWXYZ123456789",
      encodingLength: 60,
      delimiters: ["9", "8"],
      zeroDelimiterCompression: false
  };
  this.setVersion = function(e) {
      r = e
  }
  ;
  this.setLocked = function(t) {
      e = t
  }
  ;
  this.lock = function() {
      this.setLocked(true)
  }
  ;
  this.unlock = function() {
      this.setLocked(false)
  }
  ;
  this.setUrlRoot = function(e) {
      n = e
  }
  ;
  this.setTemplates = function(e) {
      a = e;
      for (var t in a) {
          this.prepare(t)
      }
      if (!this.getVersion()) {
          this.setVersion(WH.calc.executeRequired("getHashVersion"))
      }
  }
  ;
  this.setTemplate = function(e, t) {
      a[e] = t;
      this.prepare(e)
  }
  ;
  this.setEncoding = function(e, t) {
      a[e].encoding = t || s.encoding
  }
  ;
  this.setEncodingLength = function(e, t) {
      a[e].encodingLength = t || s.encodingLength
  }
  ;
  this.modifyEncodingLength = function(e, t) {
      a[e].encodingLength = a[e].encodingLength + t
  }
  ;
  this.setDelimiters = function(e, t) {
      a[e].delimiters = t || $.extend(true, [], s.delimiters)
  }
  ;
  this.setDelimiter = function(e, t, i) {
      if (!i)
          i = 1;
      a[e].delimiters[i] = t || s.delimiters[i]
  }
  ;
  this.setZeroDelimiterCompression = function(e, t) {
      a[e].zeroDelimiterCompression = t || s.zeroDelimiterCompression
  }
  ;
  this.increaseDelimiters = function(e, t) {
      while (t > 0) {
          var i = this.getEncoding(e).charAt(this.getEncodingLength(e) - 1);
          a[e].delimiters.push(i);
          this.setEncodingLength(e, this.getEncodingLength(e) - 1);
          t--
      }
  }
  ;
  this.decreaseDelimiters = function(e, t) {
      while (t > 0) {
          a[e].delimiters.pop();
          this.setEncodingLength(e, this.getEncodingLength(e) + 1);
          t--
      }
  }
  ;
  this.getVersion = function() {
      return r
  }
  ;
  this.getLocked = function() {
      return e
  }
  ;
  this.isLocked = this.getLocked;
  this.getUrlRoot = function() {
      return n
  }
  ;
  this.getTemplates = function() {
      return a
  }
  ;
  this.getTemplate = function(e, t) {
      if (t) {
          return a[e]
      }
      return a[e] || WH.calc.executeRequired("getHashTemplate")
  }
  ;
  this.getEncoding = function(e) {
      return a[e].encoding
  }
  ;
  this.getEncodingLength = function(e) {
      return a[e].encodingLength
  }
  ;
  this.getMaxEncodingIndex = function(e) {
      return this.getEncodingLength(e) - 1
  }
  ;
  this.getDelimiters = function(e) {
      return a[e].delimiters
  }
  ;
  this.getDelimiter = function(e, t) {
      if (!WH.calc.externals.isInArray(["string", "number"], typeof t) || isNaN(t)) {
          t = 1
      }
      if (typeof a[e].delimiters[t] == "undefined") {
          WH.calc.externals.error("Requested undefined delimiter.", e, t)
      }
      return a[e].delimiters[t]
  }
  ;
  this.getZeroDelimiterCompression = function(e) {
      return a[e].zeroDelimiterCompression
  }
  ;
  this.getZeroDelimiterCompressionIndicator = function(e) {
      if (typeof this.getZeroDelimiterCompression(e) != "undefined" && this.getZeroDelimiterCompression(e) !== false) {
          return this.getDelimiter(e, this.getZeroDelimiterCompression(e))
      }
  }
  ;
  this.prepare = function(e) {
      if (!a[e].prepared) {
          if (!a[e].encoding)
              this.setEncoding(e);
          if (!a[e].encodingLength)
              this.setEncodingLength(e);
          if (!a[e].delimiters)
              this.setDelimiters(e);
          if (a[e].delimiter)
              this.setDelimiter(e, a[e].delimiter);
          if (a[e].increaseDelimiters)
              this.increaseDelimiters(e, a[e].increaseDelimiters);
          if (a[e].decreaseDelimiters)
              this.decreaseDelimiters(e, a[e].decreaseDelimiters);
          if (typeof a[e].zeroDelimiterCompression == "undefined")
              this.setZeroDelimiterCompression(e);
          if (a[e].modifyEncodingLength)
              this.modifyEncodingLength(e, a[e].modifyEncodingLength);
          a[e].prepared = true
      }
  }
  ;
  this.get = function(e, t, i) {
      var r = n ? WH.getSlashHash(n) : location.hash ? ("" + location.hash).replace(/^#/, "") : "";
      if (r) {
          if (!t) {
              t = this.getVersion()
          }
          if (i === true) {
              return r
          }
          if (!e) {
              r = WH.calc.decode.zeroes(t, r)
          }
          if (typeof WH.calc.hash.getZeroDelimiterCompression(t) != "undefined" && WH.calc.hash.getZeroDelimiterCompression(t) !== false) {
              r = WH.calc.decode.zeroDelimiters(t, r)
          }
          return r
      }
      return ""
  }
  ;
  this.set = function(e, t, r) {
      if (WH.calc.hash.isLocked()) {
          WH.calc.externals.debug("WH.calc.hash.set found that the hash is locked.");
          return
      }
      if (typeof e != "string") {
          WH.calc.externals.error("Tried to update hash with non-string.", e);
          return
      }
      if (e) {
          if (!r)
              r = this.getVersion();
          if (!t) {
              e = WH.calc.encode.zeroes(r, e)
          }
          if (typeof WH.calc.hash.getZeroDelimiterCompression(r) != "undefined" && WH.calc.hash.getZeroDelimiterCompression(r) !== false) {
              e = WH.calc.encode.zeroDelimiters(r, e)
          }
          if (this.get(t, r, true) == e) {
              WH.calc.externals.debug("Tried to set hash to current value.");
              this.unlock();
              return
          }
          i++;
          if (n) {
              WH.setSlashHash(n, e, true)
          } else {
              location.hash = e
          }
      }
      this.updateLink(e)
  }
  ;
  this.update = function() {
      if (WH.calc.hash.isLocked()) {
          WH.calc.externals.debug("WH.calc.hash.update found that the hash is locked.");
          return
      }
      var e = this.getTemplate(this.getVersion());
      var t = e ? WH.calc.encode.template(e) : WH.calc.executeRequired("encodeHash");
      this.set(t)
  }
  ;
  this.removeTrailing = function(e, t) {
      return e.replace(new RegExp(t + "$"), "")
  }
  ;
  this.check = function() {
      if (WH.calc.hash.isLocked()) {
          WH.calc.externals.debug("WH.calc.hash.check found that the hash is locked.");
          return
      }
      if (i > t) {
          t++;
          return
      }
      if (i != t) {
          return
      }
      var e = this.get();
      if (!e)
          return;
      var r = this.getTemplate(this.getVersion());
      var n = r ? WH.calc.decode.template(r, e) : WH.calc.executeRequired("decodeHash", e);
      WH.calc.executeRequired("updateFromHash", n);
      this.updateLink(e)
  }
  ;
  this.updateLink = function(e) {
      if (!e)
          e = this.get();
      if (!e)
          return;
      WH.calc.executeOptional("updateLink", e)
  }
}
;
WH.calc.encode = new function() {
  this.value = function(e, t) {
      if (typeof t != "number" && !t.match(/[^0-9]/)) {
          t = parseInt(t)
      }
      if (!isNaN(t)) {
          if (t > WH.calc.hash.getMaxEncodingIndex(e)) {
              WH.calc.externals.error("calc.encode.value: Tried to encode a value higher than the encoding length.", t);
              return this.value(e, 0)
          }
          var i = WH.calc.hash.getEncoding(e).charAt(t);
          if (!i) {
              WH.calc.externals.error("calc.encode.value: Invalid result.", t);
              return this.value(e, 0)
          }
          return i
      }
      WH.calc.externals.error("calc.encode.value: Tried to encode a non-number value.", t);
      return this.value(e, 0)
  }
  ;
  this.longValue = function(e, t) {
      var i = WH.calc.hash.getMaxEncodingIndex(e);
      if (t <= i) {
          WH.calc.externals.debug("calc.encode.longValue: Using long value encoding on a short value.", t);
          return this.value(e, t)
      }
      var r = [t];
      var n = 0;
      while (r[0] > i) {
          n = Math.floor(r[0] / i);
          r[0] = r[0] - n * i;
          r.unshift(n)
      }
      var a = "";
      for (var s in r) {
          a += this.value(e, r[s])
      }
      return a
  }
  ;
  this.compressedValues = function(e, t) {
      if (typeof t != "object" || !t.base) {
          WH.calc.externals.error("calc.encode.compressedValues: Tried to encode invalid compressed values.", t);
          return this.value(e, 0)
      }
      if (typeof t.base != "number" || t.base < 1) {
          WH.calc.externals.error("calc.encode.compressedValues: Tried to encode compressed values with invalid or no base value.", t);
          return this.value(e, 0)
      }
      if (!t.data || !$.isArray(t.data) || !t.data.length) {
          WH.calc.externals.error("calc.encode.compressedValues: Tried to encode compressed values with no data.", t);
          return this.value(e, 0)
      }
      var i = 0;
      var r = 0;
      var n;
      for (var a = 0; a < t.data.length; a++) {
          if (t.data[a] > t.base) {
              WH.calc.externals.error("calc.encode.compressedValues: Value higher than the base.", t, "Problem value:", a, t.data[a]);
              return this.value(e, 0)
          }
          n = Math.pow(t.base + 1, r);
          i += t.data[a] * n;
          r++
      }
      return this.value(e, i)
  }
  ;
  this.complexCompressedValues = function(e, t) {
      var i = 0;
      for (var r = 0; r < t.length; r++) {
          if (t[r].multiplier) {
              i += t[r].value * t[r].multiplier
          } else {
              i += t[r].value
          }
      }
      return this.value(e, i)
  }
  ;
  this.values = function(e, t) {
      if (!$.isArray(t)) {
          WH.calc.externals.error("calc.encode.values: Tried to encode a non-array.", t);
          return null
      }
      var i = [];
      for (var r = 0; r < t.length; r++) {
          if (typeof t[r] == "number") {
              i.push(this.value(e, t[r]))
          } else if (typeof t[r] == "object" && t[r].compression) {
              i.push(this.compressedValues(e, t[r]))
          } else if (typeof t[r] == "string" && t[r] == "delimiter") {
              i.push(WH.calc.hash.getDelimiter(e))
          } else {
              WH.calc.externals.error("calc.encode.values: Tried to encode a value of an unhandled type.", t, "Problem value is:", t[r]);
              i.push(this.value(e, 0))
          }
      }
  }
  ;
  this.zeroes = function(e, t) {
      t = t.split("");
      var i = ""
        , r = [];
      for (var n in t) {
          if (t[n] == "0") {
              r.push("0")
          } else {
              if (r.length) {
                  if (r.length < 2) {
                      i += r.join("");
                      r = []
                  } else if (r.length > WH.calc.hash.getMaxEncodingIndex(e)) {
                      i += WH.calc.hash.getDelimiter(e, 0) + WH.calc.hash.getDelimiter(e, 0) + WH.calc.hash.getEncoding(e).charAt(r.length - WH.calc.hash.getMaxEncodingIndex(e));
                      r = []
                  } else {
                      i += WH.calc.hash.getDelimiter(e, 0) + WH.calc.hash.getEncoding(e).charAt(r.length);
                      r = []
                  }
              }
              i += t[n]
          }
      }
      return i
  }
  ;
  this.zeroDelimiters = function(e, t) {
      t = t.split("");
      var i = ""
        , r = false
        , n = [];
      for (var a = 0; a <= t.length; a++) {
          if (r) {
              r = false;
              continue
          }
          if (t[a] == "0" && t[a + 1] == WH.calc.hash.getDelimiter(e)) {
              n.push("08");
              r = true
          } else {
              if (n.length) {
                  if (n.length < 2) {
                      i += n.join("");
                      n = []
                  } else if (n.length > WH.calc.hash.getMaxEncodingIndex(e)) {
                      i += WH.calc.hash.getZeroDelimiterCompressionIndicator(e) + WH.calc.hash.getZeroDelimiterCompressionIndicator(e) + WH.calc.hash.getEncoding(e).charAt(n.length - WH.calc.hash.getMaxEncodingIndex(e));
                      n = []
                  } else {
                      i += WH.calc.hash.getZeroDelimiterCompressionIndicator(e) + WH.calc.hash.getEncoding(e).charAt(n.length);
                      n = []
                  }
              }
              if (a < t.length) {
                  i += t[a]
              }
          }
      }
      return i
  }
  ;
  this.template = function(e, t, i, r) {
      if (!e)
          e = WH.calc.hash.getTemplate(r || WH.calc.hash.getVersion());
      if (!t)
          t = WH.calc.flexibleExecuteRequired(e.build);
      if (!e || !t)
          return;
      if (!r)
          r = e.version;
      WH.calc.hash.prepare(r);
      if (e.encodingPreProcess) {
          t = WH.calc.flexibleExecuteRequired(e.encodingPreProcess, t)
      }
      var n = "";
      for (var a = 0; a < e.data.length; a++) {
          n += this.processTemplateSegment(e.data[a], t, i, r)
      }
      if (e.encodingPostProcess) {
          n = WH.calc.flexibleExecuteRequired(e.encodingPostProcess, n, t)
      }
      return n
  }
  ;
  this.processTemplateSegment = function(e, t, i, r) {
      if (WH.calc.externals.isInArray(["number", "string"], typeof e.key) || $.isArray(e.key)) {
          var n = WH.calc.externals.getValueFromObject(t, e.key);
          if (typeof n == "undefined") {
              return this.value(r, 0)
          }
          return this.value(r, n)
      }
      if (WH.calc.externals.isInArray(["number", "string"], typeof e.keyLong) || $.isArray(e.keyLong)) {
          var n = WH.calc.externals.getValueFromObject(t, e.keyLong);
          if (typeof n == "undefined") {
              return this.longValue(r, 0)
          }
          return this.longValue(r, n)
      }
      if (typeof e.compressedValue == "object") {
          var a = [];
          var s, n;
          for (var c in e.compressedValue.data) {
              s = e.compressedValue.data[c];
              if (WH.calc.externals.isInArray(["number", "string"], typeof s.key) || $.isArray(s.key)) {
                  n = WH.calc.externals.getValueFromObject(t, s.key);
                  if (typeof n == "undefined") {
                      a.push(0);
                      continue
                  }
                  a.push(n)
              }
          }
          if (a.length) {
              return this.compressedValues(r, {
                  data: a,
                  base: e.compressedValue.base
              })
          }
      }
      if ((typeof e.collectionKey == "string" || $.isArray(e.collectionKey) || WH.calc.externals.isInArray(["string", "function"], typeof e.collection)) && (WH.calc.externals.isInArray(["string", "object"], typeof e.processTemplate) || WH.calc.externals.isInArray(["string", "function"], typeof e.process))) {
          var l;
          if (typeof e.delimiter != "undefined") {
              l = WH.calc.hash.getDelimiter(r, e.delimiter)
          } else {
              l = WH.calc.hash.getDelimiter(r)
          }
          if (WH.calc.externals.isInArray(["number", "string"], typeof e.collectionKey) || $.isArray(e.collectionKey)) {
              var o = WH.calc.externals.getValueFromObject(t, e.collectionKey)
          } else {
              var o = WH.calc.flexibleExecuteRequired(e.collection, t)
          }
          if (o) {
              var n = [];
              if (WH.calc.externals.isInArray(["string", "function"], typeof e.order) || typeof e.orderKey == "string" || $.isArray(e.orderKey)) {
                  if (typeof e.orderKey == "string" || $.isArray(e.orderKey)) {
                      var u = WH.calc.externals.getValueFromObject(t, e.orderKey)
                  } else {
                      var u = WH.calc.flexibleExecuteRequired(e.order, t)
                  }
                  if (u) {
                      for (var h in u) {
                          n.push(this.processCollection(r, e, o[u[h]]))
                      }
                  }
              } else {
                  for (var f in o) {
                      n.push(this.processCollection(r, e, o[f]))
                  }
              }
              return n.join(l)
          }
      }
      if (WH.calc.externals.isInArray(["string", "function"], typeof e.calculatorValue)) {
          return this.value(r, WH.calc.flexibleExecuteRequired(e.calculatorValue))
      }
      if (WH.calc.externals.isInArray(["string", "function"], typeof e.calculatorLongValue)) {
          return this.longValue(r, WH.calc.flexibleExecuteRequired(e.calculatorLongValue))
      }
      if (typeof e.value == "number") {
          return this.value(r, e.value)
      }
      if (typeof e.longValue == "number") {
          return this.longValue(r, e.longValue)
      }
      if (e.delimiter === true)
          return WH.calc.hash.getDelimiter(r);
      if (WH.calc.externals.isInArray(["number", "string"], typeof e.delimiter)) {
          return WH.calc.hash.getDelimiter(r, e.delimiter)
      }
      if (WH.calc.externals.isInArray(["string", "function"], typeof e.func)) {
          return e.func(e, t)
      }
      WH.calc.externals.error("Invalid hash template data.", e, t);
      return this.value(r, 0)
  }
  ;
  this.processCollection = function(e, t, i) {
      if (WH.calc.externals.isInArray(["string", "object"], typeof t.processTemplate)) {
          var r = typeof t.processTemplate == "string" ? WH.calc.executeRequired("getHashTemplate", t.processTemplate) : t.processTemplate;
          return this.template(r, i, t, e)
      } else {
          return WH.calc.flexibleExecuteRequired(t.process, i)
      }
  }
}
;
WH.calc.decode = new function() {
  this.value = function(e, t) {
      if (typeof t != "number" && typeof t != "string") {
          WH.calc.externals.error("calc.decode.value: Tried to decode invalid value.", t);
          return null
      }
      var i = WH.calc.hash.getEncoding(e).indexOf(t);
      if (i == -1) {
          WH.calc.externals.error("calc.decode.value: Could not find value in encoding string.", t, WH.calc.hash.getEncoding(e));
          return null
      }
      return i
  }
  ;
  this.longValue = function(e, t) {
      if (t.length < 2) {
          WH.calc.externals.debug("calc.decode.longValue: Using long value decoding on a short value.", t);
          return this.value(e, t)
      }
      t = t.split("");
      t.reverse();
      var i = 0, r;
      for (var n = 0; n < t.length; n++) {
          r = this.value(e, t[n]);
          for (var a = 0; a < n; a++) {
              r = r * WH.calc.hash.getMaxEncodingIndex(e)
          }
          i += r
      }
      return i
  }
  ;
  this.values = function(e, t) {
      if (typeof t == "number") {
          if (isNaN(t)) {
              WH.calc.externals.error("calc.decode.values: Tried to decode NaN.", t);
              return null
          }
          t = "" + t
      }
      if (typeof t == "string") {
          t = t.split("")
      }
      if (!$.isArray(t)) {
          WH.calc.externals.error("calc.decode.values: Tried to decode invalid values.", t);
          return null
      }
      var i = [];
      for (var r = 0; r < t.length; r++) {
          i.push(this.value(e, t[r]))
      }
      return i
  }
  ;
  this.compressedValue = function(e, t, i, r, n) {
      if (!n) {
          n = [];
          for (var a = 0; a < r; a++) {
              n.push(Math.pow(t + 1, a))
          }
      }
      var s = [];
      for (var a = 0; a < n.length; a++) {
          s.push(0)
      }
      var c = this.value(e, i);
      for (a = r - 1; a >= 0; a--) {
          if (c >= n[a]) {
              s[a] = Math.floor(c / n[a]);
              c = c % n[a]
          } else {
              s[a] = 0
          }
      }
      return s
  }
  ;
  this.compressedValues = function(e, t, i, r) {
      var n = [];
      for (var a = 0; a < r; a++) {
          n.push(Math.pow(t, a))
      }
      var i = i.split("");
      var s = [];
      for (var a = 0; a < i.length; a++) {
          s.push(this.compressedValue(e, t, i[a], r, n))
      }
      return s
  }
  ;
  this.zeroes = function(e, t) {
      t = t.split("");
      var i = ""
        , r = false;
      for (var n in t) {
          if (r && t[n] == WH.calc.hash.getDelimiter(e, 0)) {
              r++
          } else if (r) {
              var a = WH.calc.hash.getEncoding(e).indexOf(t[n]) + (r - 1) * WH.calc.hash.getMaxEncodingIndex(e);
              for (n = 1; n <= a; n++) {
                  i += "0"
              }
              r = false
          } else {
              if (t[n] == WH.calc.hash.getDelimiter(e, 0)) {
                  r = 1
              } else {
                  i += t[n]
              }
          }
      }
      return i
  }
  ;
  this.zeroDelimiters = function(e, t) {
      t = t.split("");
      var i = ""
        , r = false;
      for (var n in t) {
          if (r && t[n] == WH.calc.hash.getZeroDelimiterCompressionIndicator(e)) {
              r++
          } else if (r) {
              var a = WH.calc.hash.getEncoding(e).indexOf(t[n]) + (r - 1) * WH.calc.hash.getMaxEncodingIndex(e);
              for (n = 1; n <= a; n++) {
                  i += "0" + WH.calc.hash.getDelimiter(e)
              }
              r = false
          } else {
              if (t[n] == WH.calc.hash.getZeroDelimiterCompressionIndicator(e)) {
                  r = 1
              } else {
                  i += t[n]
              }
          }
      }
      return i
  }
  ;
  this.shift = function(e, t) {
      var i = t.substr(0, 1);
      return {
          hash: t.substr(1),
          value: this.value(e, i)
      }
  }
  ;
  this.pop = function(e, t) {
      var i = t.substr(t.length - 1);
      return {
          hash: t.substr(0, t.length - 1),
          value: this.value(e, i)
      }
  }
  ;
  this.version = function(e, t) {
      var i = {
          hash: "" + t,
          build: {}
      };
      i = this.processTemplateSegment(e.data[0], e.data[1], i, null, e.version);
      if (!i.build.version) {
          return null
      }
      return i.build.version
  }
  ;
  this.template = function(e, t, i, r, n) {
      if (!e)
          e = WH.calc.hash.getTemplate(r || WH.calc.hash.getVersion());
      if (!t)
          t = WH.calc.hash.get(r);
      if (!e || !t)
          return;
      if (!r)
          r = e.version;
      WH.calc.hash.prepare(r);
      var a = t;
      if (e.decodingPreProcess) {
          a = WH.calc.flexibleExecuteRequired(e.decodingPreProcess, a)
      }
      if (!i && !n) {
          var s = this.version(e, a);
          if (s != r) {
              var c = WH.calc.hash.getTemplate(s, true);
              if (!c) {
                  WH.calc.externals.error("Build version does not match calculator version. No template found matching the build version. Attempting to use current template.", WH.calc.executeRequired("getHashVersion"), s);
                  return this.template(e, t, null, e.version, true)
              }
              WH.calc.externals.debug("Build version does not match calculator version. Retrying with old template.", WH.calc.executeRequired("getHashVersion"), s, c);
              return this.template(c, t, null, c.version)
          }
      }
      var l = {
          hash: "" + a,
          build: {}
      };
      if (n) {
          l.build.unknownTemplateVersion = true
      }
      var o = 0;
      while (l.hash.length) {
          if (!e.data[o]) {
              break
          }
          l = this.processTemplateSegment(e.data[o], e.data[o + 1], l, i, r);
          o++
      }
      if (e.decodingPostProcess) {
          l.build = WH.calc.flexibleExecuteRequired(e.decodingPostProcess, l.build, a)
      }
      return l.build
  }
  ;
  this.processTemplateSegment = function(e, t, i, r, n) {
      var a = i.hash.substr(1);
      if (WH.calc.externals.isInArray(["number", "string"], typeof e.key) || $.isArray(e.key)) {
          var s = WH.calc.externals.setValueOnObject(i.build, e.key, this.value(n, i.hash.substr(0, 1)));
          if (typeof s == "undefined") {
              WH.calc.externals.error("WH.calc.decode.processTemplateSegment: Failed to decode key.", e, t, i, s)
          }
          return {
              hash: a,
              build: s
          }
      }
      if (WH.calc.externals.isInArray(["number", "string"], typeof e.keyLong) || $.isArray(e.keyLong)) {
          var c = this.getHashPieces(n, i.hash, e, t);
          var s = WH.calc.externals.setValueOnObject(i.build, e.keyLong, this.longValue(n, c[0]));
          if (typeof s == "undefined") {
              WH.calc.externals.error("WH.calc.decode.processTemplateSegment: Failed to decode long key.", e, t, i, s)
          }
          return {
              hash: c[1] || "",
              build: s
          }
      }
      if (typeof e.compressedValue == "object") {
          var l = this.compressedValue(n, e.compressedValue.base, i.hash.substr(0, 1), e.compressedValue.data.length);
          var s = i.build;
          for (var o = 0; o < e.compressedValue.data.length; o++) {
              if (l[o]) {
                  s = WH.calc.externals.setValueOnObject(s, e.compressedValue.data[o].key, l[o])
              }
          }
          return {
              hash: a,
              build: i.build
          }
      }
      if ((typeof e.collectionKey == "string" || $.isArray(e.collectionKey) || WH.calc.externals.isInArray(["string", "function"], typeof e.collection)) && (WH.calc.externals.isInArray(["string", "object"], typeof e.processTemplate) || WH.calc.externals.isInArray(["string", "function"], typeof e.process))) {
          var u;
          if (typeof e.delimiter != "undefined") {
              u = WH.calc.hash.getDelimiter(n, e.delimiter)
          } else {
              u = WH.calc.hash.getDelimiter(n)
          }
          var h = WH.calc.hash.getDelimiter(n);
          if (u != h && i.hash.substr(0, 1) == h) {
              return {
                  hash: i.hash,
                  build: i.build
              }
          }
          var c = this.getHashPieces(n, i.hash, e, t);
          if (e.decode !== false) {
              var f = c[0].split(u);
              var d;
              var s = e.asObject ? {} : [];
              var g, p;
              for (var o = 0; o < f.length; o++) {
                  if (WH.calc.externals.isInArray(["string", "object"], typeof e.processTemplate)) {
                      d = typeof e.processTemplate == "string" ? WH.calc.executeRequired("getHashTemplate", e.processTemplate) : e.processTemplate;
                      p = this.template(d, f[o], e, n)
                  } else {
                      p = WH.calc.flexibleExecuteRequired(e.processDecoding, f[o])
                  }
                  if (WH.calc.externals.isInArray(["string", "number"], typeof e.dataAsKey)) {
                      g = p[e.dataAsKey]
                  } else {
                      g = o
                  }
                  s[g] = p
              }
              if (WH.calc.externals.isInArray(["number", "string"], typeof e.collectionKey) || $.isArray(e.collectionKey)) {
                  i.build = WH.calc.externals.setValueOnObject(i.build, e.collectionKey, s)
              } else if (WH.calc.externals.isInArray(["string", "function"], typeof e.buildFunc)) {
                  i.build = WH.calc.flexibleExecuteRequired(e.buildFunc, i.build, s, r)
              } else {
                  WH.calc.externals.error("WH.calc.decode.processTemplateSegment: Failed to set build data for collection.", e, i, s)
              }
          }
          return {
              hash: c[1] || "",
              build: i.build
          }
      }
      if (WH.calc.externals.isInArray(["string", "function"], typeof e.calculatorValue) || typeof e.value == "number") {
          if (WH.calc.externals.isInArray(["number", "string"], typeof e.buildKey) || $.isArray(e.buildKey)) {
              var s = WH.calc.externals.setValueOnObject(i.build, e.buildKey, this.value(n, i.hash.substr(0, 1)));
              if (typeof s == "undefined") {
                  WH.calc.externals.error("WH.calc.decode.processTemplateSegment: Failed to decode calculator value.", e, t, i, s)
              }
              return {
                  hash: a,
                  build: s
              }
          }
      }
      if (WH.calc.externals.isInArray(["string", "function"], typeof e.calculatorLongValue) || typeof e.longValue == "number") {
          if (WH.calc.externals.isInArray(["number", "string"], typeof e.buildKey || $.isArray(e.buildKey))) {
              var c = this.getHashPieces(n, i.hash, e, t);
              var s = WH.calc.externals.setValueOnObject(i.build, e.buildKey, this.longValue(n, c[0]));
              if (typeof s == "undefined") {
                  WH.calc.externals.error("WH.calc.decode.processTemplateSegment: Failed to decode long calculator value.", e, t, i, s)
              }
              return {
                  hash: c[1] || "",
                  build: s
              }
          }
      }
      if (e.delimiter === true && i.hash.substr(0, 1) == WH.calc.hash.getDelimiter(n) || WH.calc.externals.isInArray(["number", "string"], typeof e.delimiter) && i.hash.substr(0, 1) == WH.calc.hash.getDelimiter(n, e.delimiter)) {
          return {
              hash: a,
              build: i.build
          }
      }
      if (WH.calc.externals.isInArray(["string", "function"], typeof e.func)) {
          if (WH.calc.externals.isInArray(["string", "function"], typeof e.buildFunc)) {
              var s = WH.calc.flexibleExecuteRequired(e.buildFunc, i.build, i.hash, e, t);
              if (s) {
                  return s
              } else {
                  WH.calc.externals.error("WH.calc.decode.processTemplateSegment: Failed to decode function data.", e, t, i, s)
              }
          }
      }
      WH.calc.externals.error("Invalid hash template decoding data. Arbitrarily trimming hash by 1 character.", e, t, i);
      return {
          hash: hash.substr(1),
          build: i.build
      }
  }
  ;
  this.getHashPieces = function(e, t, i, r) {
      var n = [t];
      if (r && !r.collection && !r.collectionKey && (WH.calc.externals.isInArray(["number", "string"], typeof i.delimiter) || r.delimiter === true)) {
          var a = r.delimiter === true ? WH.calc.hash.getDelimiter(e) : WH.calc.hash.getDelimiter(e, r.delimiter);
          n = t.split(a);
          var s = [n.shift()];
          s.push(a + n.join(a));
          n = s
      }
      return n
  }
}
;
