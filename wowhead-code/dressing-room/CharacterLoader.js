function CreateCharacterLoader(e) {
  const a = WH.Wow.PlayerClass;
  var r = ["parent", "resultDiv", "resultFunc"];
  for (var t = 0; t < r.length; t++) {
      if (!e.hasOwnProperty(r[t])) {
          console.error("CreateCharacterLoader requires " + r[t] + " attribute!");
          return null
      }
  }
  var n = {
      allowCancel: false,
      characterSort: "name",
      characterRegions: 0,
      characterColumns: 2,
      sortOptions: [["name", WH.TERMS.name], ["lowlevel", WH.TERMS.lowlevel], ["highlevel", WH.TERMS.highlevel], ["realm", WH.TERMS.realm], ["classs", WH.TERMS["class"]]],
      Init: function() {
          var a = WH.ce("div", {
              id: "character-load-container-box"
          });
          WH.ae(e.parent, a);
          var r = WH.ce("div", {
              className: "character-load-container-screen"
          });
          WH.ae(a, r);
          WH.aE(r, "click", (function() {
              if (n.allowCancel) {
                  n.HideCharacterList()
              }
          }
          ));
          WH.aE(window, "keyup", (e => {
              if (e.key === "Escape" && n.allowCancel) {
                  n.HideCharacterList()
              }
          }
          ));
          var t = WH.ce("table", {
              id: "character-load-container",
              className: "infobox"
          });
          WH.ae(a, t);
          var s = WH.ce("tr");
          WH.ae(t, s);
          var c = WH.ce("th", {
              id: "characters-heading"
          });
          WH.ae(s, c);
          var i = WH.ce("div");
          WH.ae(c, i);
          i.innerHTML = WH.TERMS.yourcharacters;
          s = WH.ce("tr");
          WH.ae(t, s);
          var l = WH.ce("td");
          WH.ae(s, l);
          i = WH.ce("div", {
              id: "character-load-profiles",
              className: "no-chars"
          });
          WH.ae(l, i);
          if (WH.isRetailTree()) {
              let e = '<a href="' + WH.Strings.escapeHtml(WH.Url.generatePath("/list")) + '">';
              i.innerHTML = WH.term("noBlizzardBattleNetCharsInProfiler_format", e, "</a>")
          } else {
              WH.ae(i, WH.ce("a", {
                  className: "wrap",
                  href: WH.Url.generatePath("/client")
              }, WH.ct(WH.TERMS.useClientForCharacters_tip)))
          }
          if (WH.isRetailTree() || e.allowClassicApi && WH.isClassicTree()) {
              s = WH.ce("tr");
              WH.ae(t, s);
              c = WH.ce("th");
              WH.ae(s, c);
              c.innerHTML = WH.TERMS.loadacharacter;
              s = WH.ce("tr");
              WH.ae(t, s);
              l = WH.ce("td");
              WH.ae(s, l);
              var o = WH.ce("form", {
                  method: "POST",
                  id: "character-load-form"
              });
              WH.ae(l, o);
              WH.aE(o, "submit", n.LoadCharacter.bind(n, o));
              var H = WH.ce("select", {
                  name: "region",
                  className: "select-region",
                  ariaLabel: WH.term("choose_format", WH.TERMS.region)
              });
              WH.ae(o, H);
              WH.ae(o, WH.ce("select", {
                  name: "realm",
                  className: "select-realm",
                  ariaLabel: WH.term("choose_format", WH.TERMS.realm)
              }));
              WH.aE(H, "change", function(e) {
                  return function() {
                      n.SetRegion(e.options[e.selectedIndex].value)
                  }
              }(H));
              WH.ae(o, WH.ce("br"));
              WH.ae(o, WH.ce("input", {
                  type: "text",
                  name: "char",
                  value: "",
                  maxLength: 20,
                  required: "required",
                  placeholder: WH.TERMS.character,
                  className: "input-char",
                  ariaLabel: WH.TERMS.characterName
              }));
              WH.ae(o, WH.ce("input", {
                  type: "submit",
                  value: WH.TERMS.load,
                  className: "submit"
              }));
              var W = o.region;
              for (var u in g_bnet_realms) {
                  var f = WH.ce("option");
                  f.value = u;
                  f.label = u;
                  f.text = u;
                  W.appendChild(f);
                  if (W.options.length == 1) {
                      n.SetRegion(u)
                  }
              }
          }
          var h;
          if (WH.isRetailTree() && g_user.hasOwnProperty("lists") || !WH.isRetailTree() && g_user.hasOwnProperty("characterProfiles") && g_user.characterProfiles.length) {
              var d = WH.ce("div");
              d.className = "character-sort";
              WH.st(d, WH.TERMS.sortby_colon);
              var m = WH.ce("span");
              m.id = m.className = "character-sort-current";
              var g = [];
              for (var p = 0, v; v = n.sortOptions[p]; p++) {
                  g.push([p, v[1], n.Sort.bind(null, v[0]), undefined, {
                      checkedFunc: function(e) {
                          return n.characterSort == e[5]
                      }
                  }, v[0]]);
                  if (v[0] == n.characterSort) {
                      WH.st(m, v[1])
                  }
              }
              Menu.add(d, g, {});
              WH.ae(d, m);
              $("#characters-heading div").prepend(d);
              h = WH.ge("character-load-profiles");
              h.className = "";
              var C = 0;
              if (WH.isRetailTree()) {
                  var S = [];
                  for (var b = 0, w; w = g_user.lists[b]; b++) {
                      if (w.mode == 1) {
                          C++;
                          if (S.indexOf(w.region) < 0) {
                              S.push(w.region)
                          }
                      }
                  }
                  n.characterRegions = S.length
              } else {
                  C = (g_user.characterProfiles || []).length
              }
              if (C > 11) {
                  n.characterColumns = 4
              } else if (C > 5) {
                  n.characterColumns = 3
              }
              WH.ee(h);
              var L = WH.ce("ul");
              L.id = "character-profile-list";
              WH.ae(h, L);
              n.RenderCharacterList()
          } else if (g_user.id == 0) {
              h = WH.ge("character-load-profiles");
              if (!WH.isRetailTree()) {
                  WH.ee(h);
                  let e = location.pathname + location.search + location.hash;
                  WH.ae(h, WH.ce("div", {
                      innerHTML: WH.Strings.sprintf(WH.Strings.escapeHtml(WH.TERMS.logInToUseTool_tip), '<a href="' + WH.Url.getLoginPath(e) + '">', "</a>", '<a href="' + WH.Url.getRegistrationPath() + '">', "</a>")
                  }));
                  WH.ae(h, WH.ce("br"));
                  WH.ae(h, WH.ce("a", {
                      className: "wrap",
                      href: WH.Url.generatePath("/client")
                  }, WH.ct(WH.TERMS.useClientForCharacters_tip)))
              } else {
                  let e = WH.Url.generatePath(WH.Url.getLoginPath(window.location.pathname));
                  let a = WH.Url.generatePath("/list");
                  h.innerHTML = WH.term("loginProfiler_format", '<a href="' + WH.Strings.escapeHtml(e) + '">', "</a>", '<a href="' + WH.Strings.escapeHtml(a) + '">', "</a>")
              }
          }
          if (!e.skipFirstShow) {
              n.ShowCharacterList()
          }
          if (typeof e.hashRead == "function") {
              e.hashRead(n)
          }
      },
      trapFocus: function(e) {
          const a = e.find('a[href], button, input, select, [tabindex]:not([tabindex="-1"])').filter(":visible");
          const r = a.first();
          const t = a.last();
          e.off("keydown").on("keydown", (function(e) {
              if (e.key === "Tab") {
                  const a = e.shiftKey;
                  const n = $(document.activeElement);
                  if (a && n.is(r)) {
                      e.preventDefault();
                      t.focus()
                  } else if (!a && n.is(t)) {
                      e.preventDefault();
                      r.focus()
                  }
              }
          }
          ))
      },
      ShowCharacterList: function() {
          var a = $(e.parent);
          a.addClass("show-characters");
          var r = $("#character-load-container");
          var t = r.outerHeight(true) + 10;
          if (t > 502) {
              a.css("min-height", t + "px")
          }
          var s = r.find('a[href], button, input, select, [tabindex]:not([tabindex="-1"])').first();
          s.focus();
          n.trapFocus(r)
      },
      HideCharacterList: function() {
          $(e.parent).removeClass("show-characters").css("min-height", "")
      },
      RenderCharacterList: function() {
          var e = WH.ge("character-profile-list");
          WH.ee(e);
          var r = WH.isRetailTree();
          var t;
          if (r) {
              t = g_user.lists
          } else {
              t = g_user.characterProfiles || []
          }
          t.sort((function(e, r) {
              var t = e.classs || e["class"];
              var s = r.classs || r["class"];
              var c = n.characterSort;
              return (c == "classs" ? WH.stringCompare(a.getName(t), a.getName(s)) : 0) || (c == "realm" ? WH.stringCompare(e.region, r.region) || WH.stringCompare(e.realm, r.realm) : 0) || (c == "lowlevel" ? WH.stringCompare(e.level, r.level) : 0) || (c == "highlevel" ? WH.stringCompare(r.level, e.level) : 0) || WH.stringCompare(e.name, r.name) || WH.stringCompare(e.region, r.region) || WH.stringCompare(e.realm, r.realm)
          }
          ));
          var s = 0;
          for (var c = 0, i; i = t[c]; c++) {
              if (r && i.mode != 1) {
                  continue
              }
              var l = WH.ce("li");
              var o = WH.ce("a");
              o.href = r ? WH.Profiler.getPath(i) : "javascript:";
              WH.aE(o, "click", function(e, a) {
                  if (a.button == 0) {
                      if (r) {
                          n.LoadByUrl(a.currentTarget.href)
                      } else {
                          n.LoadLocalProfile(e)
                      }
                      return n.EventCancel(a)
                  }
              }
              .bind(null, i));
              var H = i["class"] || i.classs;
              var W = a.getIconName(H) || WH.Icon.UNKNOWN;
              var u = Icon.create(W, 0, null, "javascript:", i.level);
              $("a", u).replaceWith("<span/>");
              var f = WH.ce("span");
              f.className = "c" + H;
              WH.st(f, i.character || i.name);
              var h = WH.ce("small");
              h.innerHTML = (n.characterRegions > 1 ? i.region + " " : "") + i.realm;
              if (s % n.characterColumns == 0) {
                  l.style.clear = "left"
              }
              WH.ae(o, u);
              WH.ae(o, f);
              WH.ae(o, h);
              WH.ae(l, o);
              WH.ae(e, l);
              s++
          }
          if (r) {
              l = WH.ce("li");
              o = WH.ce("a");
              o.href = WH.Url.generatePath("/list");
              o.target = "_blank";
              f = WH.ce("span");
              WH.st(f, WH.TERMS.addacharacter_stc);
              WH.ae(o, f);
              WH.ae(l, o);
              WH.ae(e, l)
          }
      },
      Sort: function(e) {
          for (var a = 0, r; r = n.sortOptions[a]; a++) {
              if (r[0] == e) {
                  n.characterSort = e;
                  WH.st(WH.ge("character-sort-current"), r[1]);
                  n.RenderCharacterList()
              }
          }
      },
      SetRegion: function(e) {
          e = e.toUpperCase();
          if (!(e in g_bnet_realms))
              return;
          var a = WH.ge("character-load-container").getElementsByTagName("form")[0];
          var r = a.region;
          var t = a.realm;
          var n = g_bnet_realms[e];
          var s = false;
          for (var c = 0; c < r.options.length; c++)
              if (r.options[c].value == e) {
                  r.selectedIndex = c;
                  s = true
              }
          if (!s)
              return false;
          WH.ee(t);
          for (var i in n) {
              if (!n.hasOwnProperty(i)) {
                  continue
              }
              var l = WH.ce("option");
              l.value = e.toLowerCase() + "-" + n[i].slug;
              l.label = n[i].name;
              l.text = n[i].name;
              t.appendChild(l)
          }
          return true
      },
      UrlToNameRealm: function(e) {
          var a = /(us|eu|tw|kr)-(\S+)-([^-\s]+)$/i;
          var r;
          if (!(r = a.exec(e)))
              return false;
          return {
              region: r[1],
              slug: decodeURIComponent(r[2]),
              character: decodeURIComponent(r[3])
          }
      },
      LoadByUrl: function(e) {
          var a = n.UrlToNameRealm(e);
          if (!a)
              return false;
          if (!n.SetRegion(a.region))
              return false;
          var r = WH.ge("character-load-container").getElementsByTagName("form")[0];
          var t = false;
          for (var s = 0; s < r.realm.options.length; s++)
              if (r.realm.options[s].value == a.region + "-" + a.slug || r.realm.options[s].value.substr(0, a.region.length + 1) == a.region + "-" && WH.Strings.slug(r.realm.options[s].text, true) == a.slug) {
                  r.realm.selectedIndex = s;
                  t = true
              }
          if (!t) {
              WH.error("Could not find character realm!", a.slug);
              return false
          }
          r["char"].value = a.character;
          n.LoadCharacter(r);
          return true
      },
      LoadCharacter: function(a, r) {
          n.allowCancel = true;
          n.HideCharacterList();
          $("#main-contents").attr("class", "main-contents");
          if (e.onLoadCharacter) {
              e.onLoadCharacter()
          }
          WH.ee(e.resultDiv);
          var t = WH.ce("div", {
              className: "character-loading"
          });
          WH.ae(e.resultDiv, t);
          t.style.textAlign = "center";
          var s = WH.ce("img");
          WH.ae(t, s);
          s.src = WH.staticUrl + "/images/ui/misc/progress-anim.gif";
          var c = a.realm.options[a.realm.selectedIndex];
          var i = {
              region: c.value.substr(0, 2),
              realmName: c.text || c.label || c.value.substr(2),
              slug: c.value.substr(3),
              character: a["char"].value
          };
          var l = "/profile/wow/character/" + encodeURIComponent(i.slug) + "/" + encodeURIComponent(i.character.toLocaleLowerCase());
          var o = function(e, a, r) {
              var t = WH.TERMS.blizzardbattlenetfetcherror_format;
              if (e.status >= 500) {
                  t += "<br><br>" + WH.TERMS.blizzardapioutage_tip
              }
              var s = "";
              if (e.responseJSON) {
                  s = e.responseJSON.reason || e.responseJSON.detail || ""
              }
              n.ShowCharacterError(i, t, s, e.status === 404)
          };
          var H = {};
          H.namespace = "profile-";
          if (WH.isClassicTree()) {
              H.namespace += "classic1x-"
          }
          H.namespace += i.region.toLowerCase();
          H.locale = "en_US";
          WH.BlizzardApi.apiCall(i.region, l, {
              data: H,
              dataType: "json",
              method: "GET",
              success: function(a) {
                  var r = e.fields || [];
                  if (r.indexOf("status") < 0) {
                      r.push("status")
                  }
                  var t = function() {
                      if (!a.status || !a.status.is_valid) {
                          o({
                              status: 404,
                              responseJSON: {
                                  code: 404,
                                  detail: "Character Status Invalid"
                              }
                          })
                      } else {
                          e.resultFunc(a, i.region.toUpperCase(), i.slug)
                      }
                  };
                  var n = 0;
                  for (var s, c = 0; s = r[c]; c++) {
                      n++;
                      WH.BlizzardApi.apiCall(i.region, l + "/" + s, {
                          data: H,
                          dataType: "json",
                          method: "GET",
                          success: function(e, r) {
                              var s = a;
                              var c = e.replace(/-/g, "_").split("/");
                              while (c.length > 1) {
                                  var i = c.shift();
                                  if (!s.hasOwnProperty(i)) {
                                      s[i] = {}
                                  }
                                  s = s[i]
                              }
                              s[c.shift()] = r;
                              if (--n <= 0) {
                                  t()
                              }
                          }
                          .bind(null, s),
                          error: ["character-media"].includes(s) ? ( (e, a) => {
                              let r = "";
                              if (a.responseJSON) {
                                  r = a.responseJSON.reason || a.responseJSON.detail || ""
                              }
                              WH.warn(`Unable to load ${e}`, r);
                              if (--n <= 0) {
                                  t()
                              }
                          }
                          ).bind(null, l + "/" + s) : o
                      })
                  }
              },
              error: o
          });
          if (r && r.preventDefault) {
              r.preventDefault()
          }
          return false
      },
      LoadLocalProfile: function(a) {
          const r = WH.Wow.Race;
          const t = WH.Wow;
          n.allowCancel = true;
          n.HideCharacterList();
          $("#main-contents").attr("class", "main-contents");
          if (e.onLoadCharacter) {
              e.onLoadCharacter()
          }
          var s = WH.cOr({}, a);
          if (e.rawLocalProfile) {
              e.resultFunc(s, "", WH.Strings.slug(s.realm));
              return
          }
          switch (r.getSideByRaceId(s.race)) {
          case t.SIDE_ALLIANCE:
              s.faction = {
                  type: "ALLIANCE",
                  name: "Alliance"
              };
              break;
          case t.SIDE_HORDE:
              s.faction = {
                  type: "HORDE",
                  name: "Horde"
              };
              break
          }
          s.character_class = {
              id: s["class"]
          };
          s.race = {
              id: s.race
          };
          s.realm = {
              name: s.realm
          };
          s.quests = {
              completed: {
                  quests: []
              }
          };
          for (var c = 0; c < a.quests.length; c++) {
              s.quests.completed.quests.push({
                  id: a.quests[c]
              })
          }
          e.resultFunc(s, "", WH.Strings.slug(s.realm.name))
      },
      ShowCharacterError: function(a, r, t, s) {
          var c = a.character;
          c = c.substr(0, 1).toLocaleUpperCase() + c.substr(1).toLocaleLowerCase();
          $("#main-contents").attr("class", "main-contents");
          var i = e.resultDiv;
          WH.ee(i);
          $("#main-contents").attr("class", "main-contents");
          WH.ae(i, WH.getMajorHeading(WH.ct(c), 1, null, {
              classes: "character-name"
          }));
          var l = WH.ce("div");
          l.className = "error";
          l.innerHTML = WH.sprintf(r, WH.Strings.escapeHtml(c), WH.Strings.escapeHtml(a.region.toUpperCase()), WH.Strings.escapeHtml(a.realmName));
          if (t) {
              WH.ae(l, WH.ce("br"));
              WH.ae(l, WH.ce("br"));
              WH.ae(l, WH.ct(WH.TERMS.blizzarderrormessage_colon));
              var o = WH.ce("b");
              WH.st(o, t);
              WH.ae(l, o)
          }
          if (s) {
              WH.ae(l, WH.ce("br"));
              WH.ae(l, WH.ce("br"));
              WH.ae(l, WH.BlizzardApi.getNotFoundMessage())
          }
          WH.ae(l, WH.ce("br"));
          WH.ae(l, WH.ce("br"));
          WH.ae(l, new WH.Button({
              "data-variation": WH.getDataTreeKey(),
              clickHandler: n.ShowCharacterList,
              label: WH.TERMS.loadcharacter,
              size: "small",
              style: "primary"
          }).getElement());
          WH.ae(i, l);
          if (e.errorFunc) {
              e.errorFunc(a)
          }
      },
      getCharacterErrorText: function() {
          let a = e.resultDiv.querySelector(".error");
          if (!a) {
              return
          }
          a = a.cloneNode(true);
          let r;
          while (r = a.querySelector("a.btn")) {
              r.parentNode.removeChild(r)
          }
          let t = a.querySelectorAll("br,ul,li");
          for (let e, a = 0; e = t[a]; a++) {
              switch (e.nodeName) {
              case "UL":
                  e.parentNode.insertBefore(WH.ct("\n"), e);
                  break;
              case "LI":
                  e.parentNode.insertBefore(WH.ct(" - "), e);
                  break
              }
              e.parentNode.insertBefore(WH.ct("\n"), e.nextSibling)
          }
          return a.textContent.replace(/^\s+|\s+$/g, "")
      },
      EventCancel: function(e) {
          if (!e)
              if (window.event)
                  e = window.event;
              else
                  return;
          if (e.cancelBubble != null)
              e.cancelBubble = true;
          if (e.stopPropagation)
              e.stopPropagation();
          if (e.preventDefault)
              e.preventDefault();
          if (window.event)
              e.returnValue = false;
          if (e.cancel != null)
              e.cancel = true
      }
  };
  return n
}
