(function() {
  'use strict';

  var D = window.__CONFIDENCE_DATA__;
  if (!D) return;

  var app = document.getElementById('app');

  // --- Helpers ---
  function el(tag, attrs, children) {
    var e = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function(k) {
        if (k === 'style' && typeof attrs[k] === 'object') {
          Object.keys(attrs[k]).forEach(function(sk) { e.style[sk] = attrs[k][sk]; });
        } else if (k === 'className') {
          e.className = attrs[k];
        } else if (k === 'innerHTML') {
          e.innerHTML = attrs[k];
        } else {
          e.setAttribute(k, attrs[k]);
        }
      });
    }
    if (children) {
      if (!Array.isArray(children)) children = [children];
      children.forEach(function(c) {
        if (typeof c === 'string') e.appendChild(document.createTextNode(c));
        else if (c) e.appendChild(c);
      });
    }
    return e;
  }

  function pluralize(word, count) {
    return count === 1 ? word : word + 's';
  }

  function shortPath(p) {
    if (!p) return '';
    var parts = p.split('/');
    // Just the filename for most contexts
    return parts[parts.length - 1];
  }

  function pct(num, total) {
    if (total === 0) return 0;
    return Math.round(num / total * 100);
  }

  // Progressive disclosure helper: returns { visible: Element, hidden: Element|null, button: Element|null }
  function makeExpandableList(items, visibleCount, renderItem) {
    var wrap = el('div');
    var visibleItems = items.slice(0, visibleCount);
    var hiddenItems = items.slice(visibleCount);

    visibleItems.forEach(function(item) { wrap.appendChild(renderItem(item)); });

    if (hiddenItems.length > 0) {
      var expandable = el('div', { className: 'expandable' });
      hiddenItems.forEach(function(item) { expandable.appendChild(renderItem(item)); });
      wrap.appendChild(expandable);

      var btn = el('button', { className: 'show-more-btn' }, 'Show ' + hiddenItems.length + ' more');
      btn.addEventListener('click', function() {
        if (expandable.classList.contains('open')) {
          expandable.classList.remove('open');
          btn.textContent = 'Show ' + hiddenItems.length + ' more';
        } else {
          expandable.classList.add('open');
          btn.textContent = 'Show less';
        }
      });
      wrap.appendChild(btn);
    }

    return wrap;
  }

  // --- Compute derived data from raw JSON ---
  var computed = computeData(D);

  function computeData(d) {
    var c = {};

    // Style breakdown
    var behavioral = 0, structural = 0, unclassified = 0;
    var totalFakes = 0, totalMocks = 0;
    (d.fileResults || []).forEach(function(fa) {
      totalFakes += fa.doubles.fakeCount || 0;
      (fa.doubles.doubles || []).forEach(function(dd) {
        if (dd.usage !== 'fake') totalMocks++;
      });
      behavioral += fa.assertions.behavioralMethods || 0;
      structural += fa.assertions.structuralMethods || 0;
      unclassified += fa.assertions.unclassifiedMethods || 0;
    });
    var total = behavioral + structural + unclassified;
    c.totalFakes = totalFakes;
    c.totalMocks = totalMocks;
    c.hasStyle = total > 0;
    c.behavioralCount = behavioral;
    c.structuralCount = structural;
    c.unclassifiedCount = unclassified;
    if (total > 0) {
      c.behavioralPct = Math.round(behavioral / total * 100);
      c.structuralPct = Math.round(structural / total * 100);
      c.unclassifiedPct = 100 - c.behavioralPct - c.structuralPct;
    }

    // Assertion strength
    var agg = d.aggregateStrength || {};
    var strong = agg['Strong'] || agg['strong'] || 0, medium = agg['Medium'] || agg['medium'] || 0, weak = agg['Weak'] || agg['weak'] || 0;
    var totalStr = strong + medium + weak;
    c.hasAssertionStrength = totalStr > 0;
    if (totalStr > 0) {
      c.strongCount = strong; c.mediumCount = medium; c.weakCount = weak;
      c.strongPct = Math.round(strong / totalStr * 100);
      c.mediumPct = Math.round(medium / totalStr * 100);
      c.weakPct = 100 - c.strongPct - c.mediumPct;
    }

    // Findings (anti-patterns)
    // Normalize PascalCase from JSON to snake_case for lookup
    var apTypeMap = { 'ThreadSleep': 'thread_sleep', 'UnsafeDelay': 'unsafe_delay', 'EmptyTest': 'empty_test',
      'IgnoredTest': 'ignored_test', 'ConditionalLogic': 'conditional_logic', 'ReflectionUsage': 'reflection_usage',
      'GodTestClass': 'god_test_class', 'AssertionRoulette': 'assertion_roulette', 'RelaxedMockPolicy': 'relaxed_mock_policy' };
    var apCounts = {};
    (d.fileResults || []).forEach(function(fa) {
      (fa.antiPatterns || []).forEach(function(ap) {
        var t = apTypeMap[ap.type] || ap.type || '';
        apCounts[t] = (apCounts[t] || 0) + 1;
      });
    });
    var apDetails = {
      'ignored_test': { label: '@Ignored tests', why: 'These inflate the test count without testing anything. Delete or fix them.' },
      'conditional_logic': { label: 'conditional logic in tests', why: 'Branching in tests means different runs can test different things. Tests should be deterministic.' },
      'god_test_class': { label: 'god test classes', why: 'Too many methods in one class. Split by feature area for maintainability.' },
      'assertion_roulette': { label: 'assertion roulette', why: 'Too many assertions in one method. When it fails, you won\'t know which one.' },
      'thread_sleep': { label: 'sleep/delay calls', why: 'Timing-dependent tests are flaky. Use test schedulers, idling resources, or deterministic clocks.' },
      'empty_test': { label: 'empty test bodies', why: 'Tests with no logic test nothing. Delete them.' },
      'reflection_usage': { label: 'reflection in tests', why: 'Reaching past the public API to test internals. Breaks easily on refactors.' },
      'unsafe_delay': { label: 'unsafe delay patterns', why: 'Delay calls outside runTest are timing-dependent and flaky.' },
      'relaxed_mock_policy': { label: 'relaxed mock policy', why: 'Relaxed mocks silently return defaults for any call, even unexpected ones. Tests can pass when behavior has actually changed.' }
    };
    var apOrder = ['ignored_test', 'conditional_logic', 'god_test_class', 'assertion_roulette', 'thread_sleep', 'empty_test', 'reflection_usage', 'unsafe_delay', 'relaxed_mock_policy'];
    c.findings = [];
    apOrder.forEach(function(apt) {
      if (apCounts[apt] > 0) {
        var info = apDetails[apt] || { label: apt, why: '' };
        c.findings.push({ count: apCounts[apt], label: info.label, why: info.why });
      }
    });
    // Also add any types not in the predefined list
    Object.keys(apCounts).forEach(function(apt) {
      if (apOrder.indexOf(apt) < 0 && apCounts[apt] > 0) {
        var info = apDetails[apt] || { label: apt.replace(/_/g, ' '), why: '' };
        c.findings.push({ count: apCounts[apt], label: info.label, why: info.why });
      }
    });
    c.hasFindings = c.findings.length > 0;

    // Structural coupling
    var totalOutput = 0, totalInteraction = 0;
    var stCounts = {};
    var totalSetupOnly = 0, totalVerification = 0;
    (d.fileResults || []).forEach(function(fa) {
      var td = fa.assertions.targetDistribution || {};
      totalOutput += (td['Output'] || td['output'] || 0) + (td['Exception'] || td['exception'] || 0);
      totalInteraction += td['Interaction'] || td['interaction'] || 0;
      (fa.structural || []).forEach(function(si) {
        var typeMap = { 'VerifyOnlyTest': 'verify_only', 'ArgumentCaptor': 'argument_captor',
          'StubAndVerify': 'stub_and_verify', 'VerifyOrdering': 'verify_ordering',
          'PropertyAccessVerify': 'property_access_verify' };
        var st = typeMap[si.type] || si.type || '';
        stCounts[st] = (stCounts[st] || 0) + 1;
      });
      (fa.doubles.doubles || []).forEach(function(dd) {
        var usage = (dd.usage || '').toLowerCase();
        if (usage === 'fakeusage' || usage === 'fake') return;
        if (usage === 'setuponly' || usage === 'setup_only') totalSetupOnly++;
        else if (usage === 'verification') totalVerification++;
        else if (usage === 'both') { totalSetupOnly++; totalVerification++; }
      });
    });
    var totalOI = totalOutput + totalInteraction;
    c.hasStructural = totalOI > 0;
    if (totalOI > 0) {
      c.outputRatioPct = Math.round(totalOutput / totalOI * 100);
      c.interactionPct = 100 - c.outputRatioPct;
      c.outputRatioClass = c.outputRatioPct >= 85 ? 'good' : c.outputRatioPct >= 60 ? 'warn' : 'bad';
      c.interactionClass = c.interactionPct <= 15 ? 'good' : c.interactionPct <= 30 ? 'warn' : 'bad';
    }
    var totalUsage = totalSetupOnly + totalVerification;
    c.setupOnlyPct = totalUsage > 0 ? Math.round(totalSetupOnly / totalUsage * 100) : 0;
    c.stCounts = stCounts;

    // Structural indicators (including PropertyAccessVerify)
    var stLabels = {
      'verify_only': 'verify-only tests',
      'argument_captor': 'ArgumentCaptor',
      'stub_and_verify': 'stub-and-verify',
      'verify_ordering': 'verifyOrder',
      'property_access_verify': 'property-access verify'
    };
    var stOrder = ['verify_only', 'argument_captor', 'stub_and_verify', 'verify_ordering', 'property_access_verify'];
    var maxSt = 0;
    stOrder.forEach(function(st) { if ((stCounts[st] || 0) > maxSt) maxSt = stCounts[st]; });
    c.structuralIndicators = [];
    stOrder.forEach(function(st) {
      var cnt = stCounts[st] || 0;
      if (cnt > 0) {
        c.structuralIndicators.push({
          label: stLabels[st] || st, count: cnt,
          barWidth: maxSt > 0 ? Math.round(cnt / maxSt * 100) : 100
        });
      }
    });
    c.hasStructuralIndicators = c.structuralIndicators.length > 0;

    // Worst files
    var fileMap = {};
    (d.fileResults || []).forEach(function(fa) {
      var p = shortPath(fa.file.path);
      if (!fileMap[p]) fileMap[p] = { path: p, parts: [], total: 0, mockCnt: 0 };
      var fc = fileMap[p];
      (fa.structural || []).forEach(function(si) {
        var t = si.type || '';
        if (t === 'verify_only' || t === 'VerifyOnlyTest') fc.parts.push('verify-only');
        else if (t === 'argument_captor' || t === 'ArgumentCaptor') fc.parts.push('arg captor');
        else if (t === 'stub_and_verify' || t === 'StubAndVerify') fc.parts.push('stub+verify');
        else if (t === 'property_access_verify' || t === 'PropertyAccessVerify') fc.parts.push('prop-verify');
        fc.total++;
      });
      (fa.doubles.doubles || []).forEach(function(dd) {
        if (dd.usage !== 'fake') fc.mockCnt++;
      });
    });
    var worstArr = [];
    Object.keys(fileMap).forEach(function(k) { if (fileMap[k].total > 0) worstArr.push(fileMap[k]); });
    worstArr.sort(function(a, b) { return b.total - a.total; });
    c.worstFiles = worstArr.slice(0, 5).map(function(fc) {
      var detail = '';
      if (fc.mockCnt > 5) detail = fc.mockCnt + ' mocks, ';
      var typeCounts = {};
      fc.parts.forEach(function(p) { typeCounts[p] = (typeCounts[p] || 0) + 1; });
      var detailParts = [];
      Object.keys(typeCounts).forEach(function(t) { detailParts.push(typeCounts[t] + ' ' + t); });
      detail += detailParts.join(', ');
      return { name: fc.path, detail: detail, total: fc.total };
    });
    c.hasWorstFiles = c.worstFiles.length > 0;

    // Worst files note
    c.worstFilesNote = '';
    if (c.worstFiles.length >= 3) {
      var patterns = ['Analytics', 'Notification', 'Push', 'Tracking'];
      patterns.forEach(function(pat) {
        if (c.worstFilesNote) return;
        var cnt = 0;
        c.worstFiles.forEach(function(wf) { if (wf.name.indexOf(pat) >= 0) cnt++; });
        if (cnt >= 3) {
          c.worstFilesNote = cnt + ' of these are ' + pat.toLowerCase() + '-related tests. This may be an accepted pattern. Consider whether these need fakes or are legitimate boundary tests.';
        }
      });
    }

    // Mock placement
    var totalBoundary = 0, totalInternal = 0, totalUnknown = 0;
    var typePlacement = {};
    (d.fileResults || []).forEach(function(fa) {
      (fa.doubles.doubles || []).forEach(function(dd) {
        var usage = (dd.usage || '').toLowerCase();
        if (usage === 'fakeusage' || usage === 'fake') return;
        var pl = (dd.placement || '').toLowerCase();
        typePlacement[dd.typeName] = pl;
        if (pl === 'boundary') totalBoundary++;
        else if (pl === 'internal') totalInternal++;
        else totalUnknown++;
      });
    });
    var mockTotal = totalBoundary + totalInternal + totalUnknown;
    c.hasMockPlacement = mockTotal > 0;
    if (mockTotal > 0) {
      c.boundaryPct = Math.round(totalBoundary / mockTotal * 100);
      c.internalPct = Math.round(totalInternal / mockTotal * 100);
      c.unclassifiedMockPct = 100 - c.boundaryPct - c.internalPct;
    }
    c.totalInternal = totalInternal;
    c.mockTotal = mockTotal;
    c.typePlacement = typePlacement;

    // Most mocked types
    var mockCountsMap = {};
    (d.fileResults || []).forEach(function(fa) {
      (fa.doubles.doubles || []).forEach(function(dd) {
        var usg = (dd.usage || '').toLowerCase();
        if (usg !== 'fake' && usg !== 'fakeusage' && dd.typeName) {
          mockCountsMap[dd.typeName] = (mockCountsMap[dd.typeName] || 0) + 1;
        }
      });
    });
    var mockedArr = [];
    Object.keys(mockCountsMap).forEach(function(n) { mockedArr.push({ name: n, count: mockCountsMap[n] }); });
    mockedArr.sort(function(a, b) { return b.count - a.count; });
    var maxMock = mockedArr.length > 0 ? mockedArr[0].count : 0;
    c.mostMockedTypes = mockedArr.slice(0, 10).map(function(m) {
      var pl = typePlacement[m.name] || '';
      var plLabel = '', color = '#5a5854';
      if (pl === 'boundary') { plLabel = '[boundary]'; color = '#5cb87a'; }
      else if (pl === 'internal') { plLabel = '[internal]'; color = '#d4a24e'; }
      return {
        name: m.name, count: m.count, placementLabel: plLabel,
        color: color, barWidth: maxMock > 0 ? Math.round(m.count / maxMock * 100) : 100
      };
    });
    c.hasMostMocked = c.mostMockedTypes.length > 0;

    // Tautological tests
    var tautSeen = {};
    c.tautologicalTests = [];
    (d.fileResults || []).forEach(function(fa) {
      (fa.tautologies || []).forEach(function(t) {
        var key = t.method + ':' + t.stubIdentifier;
        if (tautSeen[key]) return;
        tautSeen[key] = true;
        c.tautologicalTests.push({
          file: shortPath(t.file), method: t.method,
          stubIdentifier: t.stubIdentifier, assertIdentifier: t.assertIdentifier
        });
      });
    });
    c.hasTautological = c.tautologicalTests.length > 0;
    c.tautologicalCount = c.tautologicalTests.length;

    // Surfaces
    var sa = d.surfaceAnalysis;
    c.hasSurfaces = !!(sa && sa.surfaces && sa.surfaces.length > 0);
    if (c.hasSurfaces) {
      c.surfaceTotal = sa.surfaces.length;
      c.surfaceUntested = sa.untestedCount || 0;
      c.surfaceTestedPct = c.surfaceTotal > 0 ? Math.round(sa.testedCount / c.surfaceTotal * 100) : 0;
      c.untestedSurfaces = (sa.untestedByChurn || []).map(function(sc) {
        var item = {
          name: sc.name,
          meta: sc.type + ' \u00b7 ' + (sc.lines || 0) + ' lines \u00b7 ' + (sc.functionCount || 0) + ' ' + pluralize('function', sc.functionCount || 0),
          recentChurn: sc.recentChurn || 0,
          lineCoverage: sc.lineCoverage
        };
        return item;
      });
      c.hasUntestedSurfaces = c.untestedSurfaces.length > 0;
    }

    // Untested complex files
    c.untestedComplexFiles = [];
    if (sa && sa.untestedComplexFiles && sa.untestedComplexFiles.length > 0) {
      c.untestedComplexFiles = sa.untestedComplexFiles.map(function(ucf) {
        return {
          name: ucf.name,
          meta: ucf.lines + ' lines \u00b7 ' + ucf.functionCount + ' ' + pluralize('function', ucf.functionCount),
          totalChurn: ucf.totalChurn || 0,
          recentChurn: ucf.recentChurn || 0,
          lineCoverage: ucf.lineCoverage
        };
      });
    }
    c.hasUntestedComplexFiles = c.untestedComplexFiles.length > 0;

    // Coverage summary
    var cs = d.coverageSummary;
    c.hasCoverage = !!(cs && (cs.totalLinesCovered + cs.totalLinesMissed) > 0);
    if (c.hasCoverage) {
      c.coveragePct = Math.round(cs.overallLinePct * 100);
      c.coverageLinesCovered = cs.totalLinesCovered;
      c.coverageLinesMissed = cs.totalLinesMissed;
      c.coverageFilesWithData = cs.filesWithData;
    }

    // Git signals
    var ga = d.gitAnalysis;
    c.hasGitSignals = !!(ga && ga.analyzedCommits > 0);
    if (c.hasGitSignals) {
      c.gitCommitsAnalyzed = ga.analyzedCommits;
      c.gitWorkflow = ga.workflowType || '';
      c.gitHighCoChange = 0;
      c.gitStructural = 0;
      c.gitHighChurn = 0;
      c.gitCoChangeExamples = [];
      c.gitStructuralExamples = [];
      (ga.filePairStats || []).forEach(function(fp) {
        if (fp.coChangeRatio > 0.7 && fp.prodChanges >= 3) {
          c.gitHighCoChange++;
          if (c.gitCoChangeExamples.length < 5) {
            c.gitCoChangeExamples.push(shortPath(fp.testFile) + ' (' + Math.round(fp.coChangeRatio * 100) + '% co-change, (' + fp.coChanges + '/' + fp.prodChanges + ' commits)');
          }
        }
        if (fp.smallProdChanges >= 2 && fp.structuralCouplingRate > 0.5) {
          c.gitStructural++;
          if (c.gitStructuralExamples.length < 5) {
            c.gitStructuralExamples.push(shortPath(fp.testFile) + ' (' + Math.round(fp.structuralCouplingRate * 100) + '% of small changes trigger test updates');
          }
        }
        if (fp.testChurnRatio > 1.5) c.gitHighChurn++;
      });
    }

    // Observed cost
    var cost = d.observedCost;
    c.hasObservedCost = !!(cost && cost.totalUnnecessaryChanges > 0);
    if (c.hasObservedCost) {
      c.totalUnnecessaryChanges = cost.totalUnnecessaryChanges;
      c.analyzedCommits = cost.analyzedCommits || 0;
      c.setupCoupledCount = cost.setupCoupledCount || 0;
      c.costlyTests = cost.costlyTests || [];
    }

    // Risk hotspots
    c.riskHotspots = d.riskHotspots || [];
    c.hasRiskHotspots = c.riskHotspots.length > 0;

    // Architectural patterns — cluster analysis of recurring signals
    c.archPatterns = detectArchPatterns(d, c);
    c.hasArchPatterns = c.archPatterns.length > 0;

    // Narrative
    c.narrative = buildNarrative(d, c);

    // Recommendations
    c.recommendations = buildRecs(d, c);
    c.hasRecs = c.recommendations.length > 0;

    // Trends
    c.hasTrends = !!(d.trends && d.trends.length >= 3);
    if (c.hasTrends) {
      c.trendLabels = d.trends.map(function(s) { return s.label; });
      c.trendTestCounts = d.trends.map(function(s) { return s.totalTestMethods; });
      c.trendBehavioralPcts = d.trends.map(function(s) { return Math.round(s.behavioralPct * 10) / 10; });
      c.trendStructuralRates = d.trends.map(function(s) {
        var totalSC = (s.verifyOnly || 0) + (s.argumentCaptor || 0) + (s.stubAndVerify || 0);
        var rate = s.totalTestMethods > 0 ? totalSC / s.totalTestMethods * 100 : 0;
        return Math.round(rate * 10) / 10;
      });
      c.trendFakeCounts = d.trends.map(function(s) { return s.fakeCount || 0; });
      c.trendMockCounts = d.trends.map(function(s) { return s.totalMocks || 0; });
      c.hasSurfaceTrend = false;
      c.trendSurfaceTested = d.trends.map(function(s) { return s.surfacesTested || 0; });
      c.trendSurfaceUntested = d.trends.map(function(s) { return s.surfacesUntested || 0; });
      d.trends.forEach(function(s) { if (s.surfacesTotal > 0) c.hasSurfaceTrend = true; });
    }

    return c;
  }

  // Detect architectural patterns from signal combinations
  function detectArchPatterns(d, c) {
    var patterns = [];
    var ga = d.gitAnalysis;
    var fps = (ga && ga.filePairStats) || [];

    // 1. Test Freshness / Rot — prod changed a lot, test barely changed
    var staleTests = [];
    fps.forEach(function(fp) {
      if (fp.prodChanges >= 5) {
        var ratio = fp.coChanges / fp.prodChanges;
        if (ratio < 0.2) {
          staleTests.push({ name: shortPath(fp.testFile), prodChanges: fp.prodChanges, testChanges: fp.coChanges, ratio: ratio });
        }
      }
    });
    staleTests.sort(function(a, b) { return a.ratio - b.ratio || b.prodChanges - a.prodChanges; });
    if (staleTests.length >= 3) {
      patterns.push({
        title: 'Stale Tests',
        signal: staleTests.length + ' test files have barely changed despite active production code.',
        detail: 'These tests may be passing against code they no longer exercise. The production file has evolved but the test hasn\'t kept up.',
        items: staleTests.slice(0, 5).map(function(s) {
          return s.name + ' (' + s.prodChanges + ' prod changes, ' + s.testChanges + ' test changes)';
        })
      });
    }

    // 2. Mock Blast Radius — types mocked across many files
    var mockFileCount = {};
    (d.fileResults || []).forEach(function(fa) {
      var seen = {};
      (fa.doubles.doubles || []).forEach(function(dd) {
        var usage = (dd.usage || '').toLowerCase();
        if (usage === 'fakeusage' || usage === 'fake') return;
        if (dd.typeName && !seen[dd.typeName]) {
          seen[dd.typeName] = true;
          mockFileCount[dd.typeName] = (mockFileCount[dd.typeName] || 0) + 1;
        }
      });
    });
    var blastRadius = [];
    Object.keys(mockFileCount).forEach(function(t) {
      if (mockFileCount[t] >= 4) {
        blastRadius.push({ name: t, files: mockFileCount[t] });
      }
    });
    blastRadius.sort(function(a, b) { return b.files - a.files; });
    if (blastRadius.length >= 2) {
      patterns.push({
        title: 'Mock Blast Radius',
        signal: blastRadius.length + ' mock types are used across 4+ test files.',
        detail: 'If any of these interfaces change, multiple test files break. These are coupling hubs in the test suite.',
        items: blastRadius.slice(0, 5).map(function(b) {
          return b.name + ' (used in ' + b.files + ' files)';
        })
      });
    }

    // 3. Production Complexity Shadow — high setup × high mocks × no fakes
    var shadows = [];
    (d.fileResults || []).forEach(function(fa) {
      var setup = fa.setupComplexity || {};
      var ratio = setup.ratio || 0;
      var md = fa.doubles.mockDensity || 0;
      var fakeCount = fa.doubles.fakeCount || 0;
      var totalDoubles = (fa.doubles.doubles || []).length;
      var fakeRatio = totalDoubles > 0 ? fakeCount / totalDoubles : 0;
      var shadow = ratio * md * (1 - fakeRatio);
      if (shadow > 1.0) {
        shadows.push({ name: shortPath(fa.file.path), shadow: Math.round(shadow * 10) / 10, setupRatio: Math.round(ratio * 10) / 10, mocks: totalDoubles - fakeCount });
      }
    });
    shadows.sort(function(a, b) { return b.shadow - a.shadow; });
    if (shadows.length >= 3) {
      patterns.push({
        title: 'Hard-to-Test Production Code',
        signal: shadows.length + ' test files have high setup cost, dense mocking, and no fakes.',
        detail: 'This combination usually reflects production classes with many dependencies. The test cost is a shadow of the production code\'s complexity.',
        items: shadows.slice(0, 5).map(function(s) {
          return s.name + ' (setup ratio ' + s.setupRatio + ':1, ' + s.mocks + ' mocks)';
        })
      });
    }

    // 4. Structural Decay — new tests trending more structural
    if (c.hasTrends && (d.trends || []).length >= 3) {
      var trends = d.trends;
      var first = trends[0];
      var last = trends[trends.length - 1];
      var strDelta = last.structuralPct - first.structuralPct;
      if (strDelta > 2) {
        var newMethods = last.totalTestMethods - first.totalTestMethods;
        patterns.push({
          title: 'Quality Direction',
          signal: 'Structural test ratio has increased by ' + Math.round(strDelta) + ' points over 6 months.',
          detail: newMethods > 0
            ? 'Of ' + newMethods + ' new test methods, a disproportionate share are structural. New tests are more coupled to implementation than the existing baseline.'
            : 'The ratio of structural to behavioral tests is trending unfavorably.',
          items: []
        });
      }
    }

    // 5. Refactoring Tax Rate
    if (c.hasObservedCost && ga && ga.analyzedCommits > 50) {
      var taxRate = Math.round(c.totalUnnecessaryChanges / ga.analyzedCommits * 100);
      if (taxRate >= 10) {
        patterns.push({
          title: 'Refactoring Tax',
          signal: taxRate + '% of commits required an unnecessary test change.',
          detail: 'Roughly 1 in ' + Math.round(100 / taxRate) + ' production changes forces a test update that isn\'t related to a behavior change. This is the maintenance cost of structural coupling.',
          items: []
        });
      }
    }

    // 6. False Confidence — files with tests but very low coverage
    var covSummary = d.coverageSummary;
    if (covSummary && covSummary.overallLineCoverage > 0) {
      var falseConfidence = [];
      (d.fileResults || []).forEach(function(fa) {
        var totalMethods = (fa.assertions.behavioralMethods || 0) + (fa.assertions.structuralMethods || 0);
        if (totalMethods < 3) return; // skip trivial files
        // Check if this file's production pair has coverage data
        // We look at surfaces that have tests but low coverage
      });
      // Check surfaces: tested but low coverage
      var sa = d.surfaceAnalysis;
      if (sa) {
        (sa.surfaces || []).forEach(function(s) {
          if (s.hasTest && s.lineCoverage !== undefined && s.lineCoverage !== null && s.lineCoverage < 0.1) {
            falseConfidence.push({ name: s.name, coverage: Math.round(s.lineCoverage * 100), type: s.type });
          }
        });
      }
      if (falseConfidence.length >= 3) {
        patterns.push({
          title: 'False Confidence',
          signal: falseConfidence.length + ' surfaces have test files but less than 10% line coverage.',
          detail: 'These tests exist but barely exercise the production code. This can happen when tests mock everything (so the real class never runs) or when the coverage report only covers a subset of test runs.',
          items: falseConfidence.slice(0, 5).map(function(fc) {
            return fc.name + ' (' + fc.type + ', ' + fc.coverage + '% covered)';
          })
        });
      }
    }

    return patterns;
  }

  // Build summary pills — 3 key facts for at-a-glance reading
  function buildSummaryPills(d, c) {
    var pills = [];

    // Trend direction (if available)
    if (c.hasTrends && (d.trends || []).length >= 2) {
      var first = d.trends[0];
      var last = d.trends[d.trends.length - 1];
      var behDelta = last.behavioralPct - first.behavioralPct;
      if (behDelta > 3) pills.push({ icon: '\u2197', text: 'behavioral trending up', cls: 'good' });
      else if (behDelta < -3) pills.push({ icon: '\u2198', text: 'behavioral trending down', cls: 'bad' });
      else pills.push({ icon: '\u2192', text: 'behavioral stable', cls: 'neutral' });
    }

    // Biggest coverage gap
    if (c.surfaceUntested > 0) {
      pills.push({ icon: '\u26A0', text: c.surfaceUntested + ' untested surfaces', cls: 'warn' });
    }
    if (c.hasUntestedComplexFiles) {
      pills.push({ icon: '\u26A0', text: c.untestedComplexFiles.length + ' untested logic files', cls: 'warn' });
    }

    // Line coverage
    if (c.hasCoverage) {
      pills.push({ icon: '\u2611', text: c.coveragePct + '% line coverage', cls: 'neutral' });
    }

    // Maintenance cost
    if (c.hasObservedCost) {
      pills.push({ icon: '\u23F1', text: c.totalUnnecessaryChanges + ' unnecessary test changes', cls: 'bad' });
    }

    return pills;
  }

  // Build a one-line narrative that adds trend insight (not repeating what the donut shows)
  function buildNarrative(d, c) {
    if (!c.hasTrends || (d.trends || []).length < 2) return '';

    var first = d.trends[0];
    var last = d.trends[d.trends.length - 1];
    var behDelta = last.behavioralPct - first.behavioralPct;
    var methodDelta = last.totalTestMethods - first.totalTestMethods;

    var parts = [];
    if (methodDelta > 0) {
      parts.push('The test suite has grown by <strong>' + methodDelta + ' methods</strong> over the last 6 months.');
    }
    if (behDelta > 3) {
      parts.push('The behavioral ratio has improved by <strong>' + Math.round(behDelta) + ' percentage points</strong>.');
    } else if (behDelta < -3) {
      parts.push('The behavioral ratio has dropped by <strong>' + Math.abs(Math.round(behDelta)) + ' percentage points</strong>, suggesting new tests are more structural.');
    }

    return parts.join(' ');
  }

  function pickHighestRiskSurface(surfaces) {
    if (!surfaces || surfaces.length === 0) return null;
    for (var i = 0; i < surfaces.length; i++) {
      if (surfaces[i].meta.indexOf('ViewModel') >= 0 && surfaces[i].recentChurn > 0) return surfaces[i];
    }
    for (var j = 0; j < surfaces.length; j++) {
      if (surfaces[j].recentChurn > 0) return surfaces[j];
    }
    return surfaces[0];
  }

  function buildRecs(d, c) {
    var recs = [];
    var num = 1;

    function collectExamples(type, max) {
      var examples = [], seen = {};
      (d.fileResults || []).forEach(function(fa) {
        (fa.structural || []).forEach(function(si) {
          var stTypeMap = { 'verify_only': 'VerifyOnlyTest', 'stub_and_verify': 'StubAndVerify', 'argument_captor': 'ArgumentCaptor', 'property_access_verify': 'PropertyAccessVerify' };
          if (si.type === type || si.type === stTypeMap[type]) {
            var key = shortPath(si.file) + ':' + si.method;
            if (!seen[key]) {
              seen[key] = true;
              examples.push(shortPath(si.file) + ': ' + si.method);
            }
          }
        });
      });
      return examples.slice(0, max);
    }

    function collectInternalMockExamples(max) {
      var typeCounts = {};
      (d.fileResults || []).forEach(function(fa) {
        (fa.doubles.doubles || []).forEach(function(dd) {
          var usg2 = (dd.usage || '').toLowerCase();
          var pl2 = (dd.placement || '').toLowerCase();
          if (usg2 !== 'fake' && usg2 !== 'fakeusage' && pl2 === 'internal') {
            typeCounts[dd.typeName] = (typeCounts[dd.typeName] || 0) + 1;
          }
        });
      });
      var sorted = [];
      Object.keys(typeCounts).forEach(function(n) { sorted.push({ name: n, count: typeCounts[n] }); });
      sorted.sort(function(a, b) { return b.count - a.count; });
      return sorted.slice(0, max).map(function(s) { return s.name + ' (mocked ' + s.count + 'x)'; });
    }

    var stubVerifyExamples = collectExamples('stub_and_verify', 5);
    var internalMockExamples = collectInternalMockExamples(5);

    // Surface coverage rec
    var sa = d.surfaceAnalysis;
    if (sa && sa.untestedCount > 3) {
      var untestedHTML = (sa.untestedByChurn || []).map(function(sc) {
        return {
          name: sc.name,
          meta: sc.type + ' \u00b7 ' + (sc.lines || 0) + ' lines \u00b7 ' + (sc.functionCount || 0) + ' ' + pluralize('function', sc.functionCount || 0),
          recentChurn: sc.recentChurn || 0
        };
      });
      var topSurface = pickHighestRiskSurface(untestedHTML);
      var top = '';
      if (topSurface) top = topSurface.name + ' (' + topSurface.meta + ')';
      var surfaceExamples = [];
      untestedHTML.slice(0, 3).forEach(function(s) {
        var detail = s.name;
        if (s.recentChurn > 0) detail += ', ' + s.recentChurn + ' commits';
        surfaceExamples.push(detail);
      });
      var title = sa.untestedCount + ' surfaces have no tests';
      if (top) title += '. Highest risk: ' + top;
      recs.push({
        number: num, title: title,
        cost: 'Bugs in untested surfaces are caught in production, not CI.',
        fix: 'Start with a behavioral test: assert on observable state. For ViewModels, provide fakes, call a method, and assert on the <code>StateFlow</code> value.',
        examples: surfaceExamples
      });
      num++;
    }

    // Co-change / fragile tests rec
    if (c.hasObservedCost && c.totalUnnecessaryChanges > 5) {
      var coChangeExamples = c.costlyTests.slice(0, 3).map(function(ct) { return ct.name + ' (' + ct.unnecessaryChanges + ' breaks, ' + (ct.coChangeRate || 0) + '% co-change)'; });
      recs.push({
        number: num,
        title: c.totalUnnecessaryChanges + ' test changes triggered by small code tweaks',
        cost: 'Every minor refactor forces test updates, creating friction against improving code.',
        fix: 'These tests likely use verify calls or mirror implementation structure. Rewrite to assert on outputs: given input X, expect output Y.',
        examples: coChangeExamples
      });
      num++;
    }

    // Verify-only rec
    var verifyOnlyCount = c.stCounts['verify_only'] || 0;
    if (verifyOnlyCount > 3) {
      recs.push({
        number: num,
        title: verifyOnlyCount + ' test methods contain only verify calls with no output assertions',
        cost: 'These tests check that code was called, not that it worked. A passing test does not mean the feature works.',
        fix: 'Add assertions on the observable result. If the method has side effects, use a Fake that captures the effect and assert on it.',
        examples: collectExamples('verify_only', 5)
      });
      num++;
    }

    // Stub-and-verify rec
    if ((c.stCounts['stub_and_verify'] || 0) > 3) {
      recs.push({
        number: num,
        title: c.stCounts['stub_and_verify'] + ' tests stub and verify the same method',
        cost: 'These tests break on every refactor because they assert on wiring, not behavior.',
        fix: 'Replace <code>verify { repo.save(x) }</code> with a Fake: <code>assertThat(fakeRepo.saved).contains(x)</code>',
        examples: stubVerifyExamples
      });
      num++;
    }

    // Internal mocks rec
    if (c.mockTotal > 0) {
      var internalPct = c.totalInternal / c.mockTotal * 100;
      if (internalPct > 50) {
        recs.push({
          number: num,
          title: Math.round(internalPct) + '% of mocks are on internal types',
          cost: 'Mocking internals couples tests to implementation. Refactoring breaks tests even when behavior is unchanged.',
          fix: 'Replace <code>mockk&lt;GetUser&gt;()</code> with <code>FakeGetUser</code> that returns test data. Only mock at boundaries.',
          examples: internalMockExamples
        });
        num++;
      }
    }

    return recs;
  }

  // --- Section explainers ---
  // Each section gets a short paragraph explaining what it measures, why it matters,
  // and how to interpret the results. Educational, not prescriptive.
  var explainers = {
    hero: '',
    actions: 'Ranked by how much risk or maintenance cost each change would reduce.',
    cost: 'How often a small code change (under 10 lines) also forced a test change. High numbers suggest tests are coupled to implementation details.',
    hotspots: 'Files that change often and have fragile tests. These are the most likely to cause CI failures.',
    coverage: 'Code with no test coverage. Bugs here are caught by users, not CI.',
    findings: 'Patterns that make tests flaky, fragile, or misleading.',
    structural: 'Tests that depend on <em>how</em> code works rather than <em>what</em> it produces. These cost more to maintain the more often the code is refactored.',
    trends: 'How the test suite has changed over the last 6 months, reconstructed from git history.',
    patterns: 'Recurring patterns detected by combining multiple signals. These observations connect test quality problems to their likely root causes.',
    mocks: 'Boundary mocks (network, database) are standard. Internal mocks (use cases, coordinators, ViewModels) couple tests to how components are wired together.'
  };

  function addExplainer(parent, key) {
    if (explainers[key]) {
      parent.appendChild(el('p', { style: { fontSize: '13px', color: 'var(--text-dim)', lineHeight: '1.5', marginBottom: '20px' }, innerHTML: explainers[key] }));
    }
  }

  // --- Render ---
  var container = el('div', { className: 'container' });
  app.appendChild(container);

  // Timestamp
  var now = new Date();
  var months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  var timestamp = now.getDate() + ' ' + months[now.getMonth()] + ' ' + now.getFullYear() + ' ' + ('0'+now.getHours()).slice(-2) + ':' + ('0'+now.getMinutes()).slice(-2);

  // Hero
  var hero = el('div', { className: 'hero' });
  hero.appendChild(el('div', { className: 'hero-label' }, 'Confidence Report'));
  // Use the page title (which is already shortened by the Go side) or fall back to shortPath
  var projectName = document.title.replace('Confidence — ', '') || shortPath(D.path || '');
  hero.appendChild(el('h1', null, projectName));

  if (computed.hasStyle) {
    // Hero layout: centered donut with stats below
    var heroVisual = el('div', { style: { display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '48px', margin: '40px 0 32px' } });

    // Donut chart — large and prominent
    var donutWrap = el('div', { style: { position: 'relative', width: '220px', height: '220px', flexShrink: '0' } });
    var donutCanvas = el('canvas', { id: 'heroDonut' });
    donutWrap.appendChild(donutCanvas);
    var donutCenter = el('div', { style: { position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', textAlign: 'center' } });
    donutCenter.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '40px', fontWeight: '700', color: 'var(--text)', lineHeight: '1' } }, computed.behavioralPct + '%'));
    donutCenter.appendChild(el('div', { style: { fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' } }, 'behavioral'));
    donutWrap.appendChild(donutCenter);
    heroVisual.appendChild(donutWrap);

    // Legend + stats next to the donut
    var heroRight = el('div', { style: { display: 'flex', flexDirection: 'column', gap: '16px' } });
    var legendItems = [
      { color: '#5cb87a', label: 'Behavioral', pct: computed.behavioralPct },
      { color: '#d4a24e', label: 'Structural', pct: computed.structuralPct }
    ];
    if (computed.unclassifiedPct > 0) legendItems.push({ color: '#5a5854', label: 'Unclassified', pct: computed.unclassifiedPct });
    legendItems.forEach(function(item) {
      var row = el('div', { style: { display: 'flex', alignItems: 'center', gap: '10px' } });
      row.appendChild(el('div', { style: { width: '12px', height: '12px', borderRadius: '3px', background: item.color, flexShrink: '0' } }));
      row.appendChild(el('span', { style: { fontSize: '14px', color: 'var(--text-muted)', minWidth: '100px' } }, item.label));
      row.appendChild(el('span', { style: { fontFamily: 'var(--mono)', fontSize: '16px', fontWeight: '700', color: 'var(--text)' } }, item.pct + '%'));
      heroRight.appendChild(row);
    });
    // Stats row
    var statsRow = el('div', { style: { display: 'flex', gap: '24px', marginTop: '8px', paddingTop: '16px', borderTop: '1px solid var(--bg-elevated)' } });
    statsRow.appendChild(makeStat(D.totalTestFiles, 'files'));
    statsRow.appendChild(makeStat(D.totalTestMethods, 'methods'));
    if (computed.hasCoverage) {
      statsRow.appendChild(makeStat(computed.coveragePct + '%', 'line coverage'));
    }
    statsRow.appendChild(makeStat(D.platform, 'platform'));
    heroRight.appendChild(statsRow);
    heroVisual.appendChild(heroRight);
    hero.appendChild(heroVisual);

    // Explainer as a subtle subtitle
    hero.appendChild(el('p', { style: { fontSize: '13px', color: 'var(--text-dim)', lineHeight: '1.5', maxWidth: '680px', margin: '0 auto', textAlign: 'center' } },
      'Behavioral tests check what code produces. Structural tests check how it works internally. ' +
      'Behavioral tests survive refactoring; structural tests tend to break when implementation changes.'));

    // Summary pills — 3-second read
    var pills = buildSummaryPills(D, computed);
    if (pills.length > 0) {
      var pillRow = el('div', { style: { display: 'flex', justifyContent: 'center', gap: '12px', flexWrap: 'wrap', marginTop: '20px' } });
      pills.forEach(function(p) {
        var colors = { good: 'var(--green)', bad: 'var(--red)', warn: 'var(--accent)', neutral: 'var(--text-muted)' };
        var bgs = { good: 'var(--green-dim)', bad: 'var(--red-dim)', warn: 'var(--accent-dim)', neutral: 'var(--bg-elevated)' };
        var pill = el('span', { style: {
          fontFamily: 'var(--mono)', fontSize: '12px', color: colors[p.cls] || 'var(--text-muted)',
          background: bgs[p.cls] || 'var(--bg-elevated)', padding: '5px 14px', borderRadius: '20px'
        } }, p.icon + ' ' + p.text);
        pillRow.appendChild(pill);
      });
      hero.appendChild(pillRow);
    }
  } else {
    var heroStats = el('div', { className: 'hero-stats' });
    heroStats.appendChild(makeStat(D.totalTestFiles, 'test files'));
    heroStats.appendChild(makeStat(D.totalTestMethods, 'methods'));
    heroStats.appendChild(makeStat(D.platform, 'platform'));
    hero.appendChild(heroStats);
  }
  container.appendChild(hero);

  function makeStat(value, label) {
    var stat = el('div', { className: 'hero-stat' });
    stat.appendChild(el('span', { className: 'value' }, String(value)));
    stat.appendChild(el('span', { className: 'label' }, label));
    return stat;
  }

  // Trends — with narrative insight as intro
  if (computed.hasTrends) {
    var trendSection = el('div', { className: 'section' });
    var trendHeader = el('div', { className: 'section-header' });
    trendHeader.appendChild(el('h2', { id: 'trends' }, 'Direction'));
    trendHeader.appendChild(el('span', { className: 'badge' }, 'Last 6 months'));
    trendSection.appendChild(trendHeader);
    if (computed.narrative) {
      trendSection.appendChild(el('p', { style: { fontSize: '15px', color: 'var(--text-muted)', lineHeight: '1.6', marginBottom: '20px' }, innerHTML: computed.narrative }));
    } else {
      addExplainer(trendSection, 'trends');
    }
    var trendGrid = el('div', { style: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px' } });
    trendGrid.appendChild(makeTrendChartWrap('Suite Growth', 'trendGrowthChart',
      'Total test files and methods over time.'));
    trendGrid.appendChild(makeTrendChartWrap('Quality Direction', 'trendQualityChart',
      'Behavioral vs structural balance. A rising behavioral line means more tests check outcomes rather than implementation.'));
    trendGrid.appendChild(makeTrendChartWrap('Test Doubles Strategy', 'trendDoublesChart',
      'Fakes vs mocks over time. Fakes are self-contained; mocks couple tests to internal method calls.'));
    if (computed.hasSurfaceTrend) {
      trendGrid.appendChild(makeTrendChartWrap('Coverage Progress', 'trendCoverageChart',
        'Tested vs untested surfaces. Watch whether tested count keeps pace as new surfaces are added.'));
    }
    trendSection.appendChild(trendGrid);
    container.appendChild(trendSection);
  }

  function makeTrendChartWrap(title, id, subtitle) {
    var wrap = el('div');
    wrap.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '4px' } }, title));
    if (subtitle) {
      wrap.appendChild(el('div', { style: { fontSize: '12px', color: 'var(--text-dim)', marginBottom: '12px', lineHeight: '1.4' } }, subtitle));
    }
    var chartFull = el('div', { className: 'chart-full', style: { maxHeight: '280px' } });
    chartFull.appendChild(el('canvas', { id: id }));
    wrap.appendChild(chartFull);
    return wrap;
  }

  // Section nav
  var nav = el('nav', { style: { display: 'flex', flexWrap: 'wrap', gap: '8px', marginBottom: '48px', marginTop: '16px', fontFamily: 'var(--mono)', fontSize: '11px', letterSpacing: '0.5px' } });
  var navItems = [];
  if (computed.hasRecs) navItems.push(['#actions', 'Actions']);
  if (computed.hasObservedCost || computed.hasRiskHotspots) navItems.push(['#evidence', 'Evidence']);
  if (computed.hasSurfaces || computed.hasUntestedComplexFiles) navItems.push(['#coverage', 'Coverage']);
  if (computed.hasArchPatterns || computed.hasStructural || computed.hasFindings || computed.hasMockPlacement) navItems.push(['#patterns', 'Patterns']);
  navItems.forEach(function(item) {
    nav.appendChild(el('a', { href: item[0], style: { color: 'var(--text-muted)', textDecoration: 'none', padding: '6px 14px', border: '1px solid var(--bg-elevated)', borderRadius: '6px', background: 'var(--bg-card)', fontSize: '12px', transition: 'border-color 0.2s' } }, item[1]));
  });
  container.appendChild(nav);

  // === SECTION ORDER: Actions → Cost → Hotspots → Coverage → Findings → Coupling → Trends → Mocks ===

  // 1. Where To Start (most actionable — moved to top)
  if (computed.hasRecs) {
    var recSection = el('div', { className: 'section' });
    var recHeader = el('div', { className: 'section-header' });
    recHeader.appendChild(el('h2', { id: 'actions' }, 'Where To Start'));
    recSection.appendChild(recHeader);
    addExplainer(recSection, 'actions');
    computed.recommendations.forEach(function(r) {
      var rec = el('div', { className: 'rec' });
      rec.appendChild(el('div', { className: 'rec-number' }, 'Priority ' + r.number));
      rec.appendChild(el('div', { className: 'rec-title' }, r.title));
      rec.appendChild(el('div', { className: 'rec-cost' }, r.cost));
      rec.appendChild(el('div', { className: 'rec-fix', innerHTML: r.fix }));
      if (r.examples && r.examples.length > 0) {
        rec.appendChild(makeDetails('Show ' + r.examples.length + ' ' + pluralize('example', r.examples.length), r.examples));
      }
      recSection.appendChild(rec);
    });
    container.appendChild(recSection);
  }

  // 2. Evidence (merge Cost + Hotspots)
  if (computed.hasObservedCost || computed.hasRiskHotspots) {
    var evSection = el('div', { className: 'section' });
    var evHeader = el('div', { className: 'section-header' });
    evHeader.appendChild(el('h2', { id: 'evidence' }, 'Evidence'));
    evSection.appendChild(evHeader);
    addExplainer(evSection, 'cost');

    // Observed cost subsection
    if (computed.hasObservedCost) {
      var costSummary = '<strong style="color: var(--accent);">' + computed.totalUnnecessaryChanges + '</strong> unnecessary test changes across ' + computed.analyzedCommits + ' commits.';
      if (computed.setupCoupledCount > 0) costSummary += ' ' + computed.setupCoupledCount + ' are behavioral tests likely coupled through mock setup.';
      evSection.appendChild(el('p', { style: { fontSize: '15px', color: 'var(--text-muted)', marginBottom: '16px' }, innerHTML: costSummary }));
      var costListWrap = makeExpandableList(computed.costlyTests, 3, function(ct) {
        var row = el('div', { style: { display: 'grid', gridTemplateColumns: '1fr 60px', alignItems: 'center', gap: '12px', padding: '10px 0', borderBottom: '1px solid var(--bg-elevated)' } });
        var left = el('div');
        left.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '13px', color: 'var(--text)' } }, ct.name));
        left.appendChild(el('div', { style: { fontSize: '12px', color: 'var(--text-dim)' } }, (ct.coChangeRate || 0) + '% co-change rate'));
        row.appendChild(left);
        var right = el('div', { style: { textAlign: 'right' } });
        right.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '20px', fontWeight: '700', color: 'var(--red)' } }, String(ct.unnecessaryChanges)));
        right.appendChild(el('div', { style: { fontSize: '10px', color: 'var(--text-dim)' } }, 'breaks'));
        row.appendChild(right);
        return row;
      });
      evSection.appendChild(costListWrap);
    }

    // Risk hotspots subsection
    if (computed.hasRiskHotspots) {
      evSection.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px', marginTop: '28px' } }, 'Risk Hotspots'));
      evSection.appendChild(el('p', { style: { fontSize: '13px', color: 'var(--text-dim)', marginBottom: '16px' } }, 'Production files that change often and have fragile test coverage.'));
      var hsRank = 0;
      var hsListWrap = makeExpandableList(computed.riskHotspots, 3, function(rh) {
        hsRank++;
        var card = el('div', { className: 'rec' });
        var titleRow = el('div', { style: { display: 'flex', alignItems: 'baseline', gap: '12px' } });
        titleRow.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '14px', fontWeight: '700', color: 'var(--accent)', minWidth: '24px' } }, '#' + hsRank));
        titleRow.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '13px', color: 'var(--text)', wordBreak: 'break-all' } }, shortPath(rh.productionFile)));
        card.appendChild(titleRow);
        var detailParts = [];
        if (rh.commits) detailParts.push(rh.commits + ' ' + pluralize('commit', rh.commits));
        if (rh.couplingScore) detailParts.push('coupling score ' + Math.round(rh.couplingScore));
        if (rh.verifyOnly) detailParts.push(rh.verifyOnly + ' verify-only');
        if (rh.tautologies) detailParts.push(rh.tautologies + ' pass-through');
        if (detailParts.length > 0) {
          card.appendChild(el('div', { style: { fontSize: '12px', color: 'var(--text-muted)', marginTop: '8px', fontFamily: 'var(--mono)' } }, detailParts.join(' \u00b7 ')));
        }
        if (rh.costHint) card.appendChild(el('div', { className: 'rec-cost', style: { marginTop: '8px' } }, rh.costHint));
        if (rh.fixHint) card.appendChild(el('div', { className: 'rec-fix', style: { marginTop: '4px' }, innerHTML: rh.fixHint }));
        return card;
      });
      evSection.appendChild(hsListWrap);
    }
    container.appendChild(evSection);
  }

  // 3. Coverage Gaps (surfaces + untested business logic)
  if (computed.hasSurfaces || computed.hasUntestedComplexFiles) {
    var covSec = el('div', { className: 'section' });
    var covHdr = el('div', { className: 'section-header' });
    covHdr.appendChild(el('h2', { id: 'coverage' }, 'Coverage Gaps'));
    covSec.appendChild(covHdr);
    addExplainer(covSec, 'coverage');

    // Untested surfaces
    if (computed.hasSurfaces) {
      covSec.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px' } }, 'User-Facing Surfaces'));
      var surfMetrics = el('div', { className: 'metrics' });
      surfMetrics.appendChild(makeMetricCard('neutral', computed.surfaceTotal, 'surfaces'));
      surfMetrics.appendChild(makeMetricCard('good', computed.surfaceTestedPct + '%', 'tested'));
      surfMetrics.appendChild(makeMetricCard('neutral', computed.surfaceUntested, 'untested'));
      covSec.appendChild(surfMetrics);
      var progressTrack = el('div', { className: 'progress-bar-track' });
      progressTrack.appendChild(el('div', { className: 'progress-bar-fill', style: { width: computed.surfaceTestedPct + '%' } }));
      covSec.appendChild(progressTrack);
      if (computed.hasUntestedSurfaces) {
        var surfListWrap = makeExpandableList(computed.untestedSurfaces, 5, function(s) {
          var li = el('div', { className: 'surface-item' });
          var left = el('div');
          left.appendChild(el('div', { className: 'surface-name' }, s.name));
          left.appendChild(el('div', { className: 'surface-meta' }, s.meta));
          li.appendChild(left);
          var right = el('div');
          right.appendChild(el('span', { className: 'surface-badge untested' }, 'no tests'));
          if (s.lineCoverage != null) {
            var covPctVal = Math.round(s.lineCoverage * 100);
            var covCls = covPctVal >= 80 ? 'good' : covPctVal >= 50 ? 'warn' : 'bad';
            var covColors = { good: '#5cb87a', warn: '#d4a24e', bad: '#e05252' };
            right.appendChild(el('span', { className: 'surface-badge', style: { color: covColors[covCls], borderColor: covColors[covCls] } }, covPctVal + '% covered'));
          }
          if (s.recentChurn > 0) right.appendChild(el('span', { className: 'surface-badge churn' }, s.recentChurn + ' ' + pluralize('commit', s.recentChurn) + ' (3mo)'));
          li.appendChild(right);
          return li;
        });
        covSec.appendChild(surfListWrap);
      }
    }

    // Untested business logic
    if (computed.hasUntestedComplexFiles) {
      covSec.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px', marginTop: '28px' } }, 'Business Logic (' + computed.untestedComplexFiles.length + ' files)'));
      covSec.appendChild(el('p', { style: { fontSize: '13px', color: 'var(--text-muted)', marginBottom: '16px' } }, 'Production files with significant complexity and git churn but no test coverage.'));
      var ucfListWrap = makeExpandableList(computed.untestedComplexFiles, 5, function(ucf) {
        var item = el('div', { className: 'surface-item' });
        var left = el('div');
        left.appendChild(el('div', { className: 'surface-name' }, ucf.name));
        left.appendChild(el('div', { className: 'surface-meta' }, ucf.meta));
        item.appendChild(left);
        var right = el('div');
        right.appendChild(el('span', { className: 'surface-badge untested' }, 'no tests'));
        if (ucf.lineCoverage != null) {
          var ucfCovPct = Math.round(ucf.lineCoverage * 100);
          var ucfCovCls = ucfCovPct >= 80 ? 'good' : ucfCovPct >= 50 ? 'warn' : 'bad';
          var ucfCovColors = { good: '#5cb87a', warn: '#d4a24e', bad: '#e05252' };
          right.appendChild(el('span', { className: 'surface-badge', style: { color: ucfCovColors[ucfCovCls], borderColor: ucfCovColors[ucfCovCls] } }, ucfCovPct + '% covered'));
        }
        if (ucf.recentChurn > 0) right.appendChild(el('span', { className: 'surface-badge churn' }, ucf.recentChurn + ' recent ' + pluralize('commit', ucf.recentChurn)));
        else if (ucf.totalChurn > 0) right.appendChild(el('span', { className: 'surface-badge churn' }, ucf.totalChurn + ' ' + pluralize('commit', ucf.totalChurn)));
        item.appendChild(right);
        return item;
      });
      covSec.appendChild(ucfListWrap);
    }

    container.appendChild(covSec);
  }

  // 4. Patterns (merge: novel signals + structural + findings + mocks)
  if (computed.hasArchPatterns || computed.hasStructural || computed.hasFindings || computed.hasTautological || computed.hasMockPlacement) {
    var patSection = el('div', { className: 'section' });
    var patHeader = el('div', { className: 'section-header' });
    patHeader.appendChild(el('h2', { id: 'patterns' }, 'Patterns'));
    patSection.appendChild(patHeader);
    addExplainer(patSection, 'patterns');

    // Novel signal combinations (blast radius, complexity shadow, etc.)
    if (computed.hasArchPatterns) {
      computed.archPatterns.forEach(function(pat) {
        var card = el('div', { className: 'rec' });
        card.appendChild(el('div', { className: 'rec-title' }, pat.title));
        card.appendChild(el('div', { style: { fontSize: '14px', color: 'var(--text)', marginBottom: '6px' } }, pat.signal));
        card.appendChild(el('div', { style: { fontSize: '13px', color: 'var(--text-dim)' } }, pat.detail));
        if (pat.items && pat.items.length > 0) {
          var itemList = el('div', { style: { marginTop: '10px' } });
          pat.items.forEach(function(item) {
            itemList.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '12px', color: 'var(--text-muted)', padding: '3px 0' } }, item));
          });
          card.appendChild(itemList);
        }
        patSection.appendChild(card);
      });
    }

    // Structural coupling indicators
    if (computed.hasStructural) {
      patSection.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px', marginTop: '28px' } }, 'Structural Coupling'));
      var strMetrics = el('div', { className: 'metrics' });
      strMetrics.appendChild(makeMetricCard(computed.outputRatioClass, computed.outputRatioPct + '%', 'output assertions'));
      strMetrics.appendChild(makeMetricCard(computed.interactionClass, computed.interactionPct + '%', 'verify calls'));
      strMetrics.appendChild(makeMetricCard('neutral', computed.setupOnlyPct + '%', 'setup-only doubles'));
      patSection.appendChild(strMetrics);

      if (computed.hasStructuralIndicators) {
        var indWrap = el('div', { style: { marginTop: '16px' } });
        computed.structuralIndicators.forEach(function(si) {
          var row = el('div', { style: { display: 'grid', gridTemplateColumns: '160px 1fr 50px', alignItems: 'center', gap: '12px', marginBottom: '10px' } });
          row.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '12px', color: 'var(--text-muted)', textAlign: 'right' } }, si.label));
          var barTrack = el('div', { style: { height: '24px', background: 'var(--bg-elevated)', borderRadius: '4px', overflow: 'hidden', position: 'relative' } });
          barTrack.appendChild(el('div', { style: { height: '100%', width: si.barWidth + '%', background: 'var(--accent)', borderRadius: '4px', opacity: '0.85' } }));
          row.appendChild(barTrack);
          row.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '14px', fontWeight: '700', color: 'var(--text)' } }, String(si.count)));
          indWrap.appendChild(row);
        });
        patSection.appendChild(indWrap);
      }

      if (computed.hasWorstFiles) {
        patSection.appendChild(makeDetails('Show worst files (' + computed.worstFiles.length + ')', computed.worstFiles.map(function(wf) { return wf.name + ' (' + wf.total + ' ' + pluralize('indicator', wf.total) + ')'; })));
      }
    }

    // Anti-patterns (collapsible)
    if (computed.hasFindings) {
      patSection.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px', marginTop: '28px' } }, 'Anti-patterns'));
      var findingsWrap = makeExpandableList(computed.findings, 4, function(f) {
        var row = el('div', { style: { display: 'flex', alignItems: 'baseline', gap: '12px', padding: '8px 0', borderBottom: '1px solid var(--bg-elevated)' } });
        row.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '20px', fontWeight: '700', color: 'var(--accent)', minWidth: '36px' } }, String(f.count)));
        var right = el('div');
        right.appendChild(el('div', { style: { color: 'var(--text)', fontWeight: '500', fontSize: '14px' } }, f.label));
        right.appendChild(el('div', { style: { color: 'var(--text-dim)', fontSize: '12px' } }, f.why));
        row.appendChild(right);
        return row;
      });
      patSection.appendChild(findingsWrap);
    }

    // Pass-through tests (collapsible)
    if (computed.hasTautological) {
      patSection.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px', marginTop: '28px' } }, 'Pass-Through Tests (' + computed.tautologicalCount + ')'));
      patSection.appendChild(el('p', { style: { fontSize: '12px', color: 'var(--text-dim)', marginBottom: '12px' } }, 'Tests that assert the same value they stubbed, so no real logic is being tested.'));
      var tautWrap = makeExpandableList(computed.tautologicalTests, 3, function(t) {
        var card = el('div', { style: { background: 'var(--bg-card)', borderRadius: 'var(--radius)', border: '1px solid var(--bg-elevated)', padding: '12px 16px', marginBottom: '6px' } });
        card.appendChild(el('div', { className: 'surface-name' }, t.method));
        card.appendChild(el('div', { className: 'surface-meta' }, t.file));
        return card;
      });
      patSection.appendChild(tautWrap);
    }

    // Mock placement (inline, no separate section)
    if (computed.hasMockPlacement) {
      patSection.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px', marginTop: '28px' } }, 'Mock Placement'));
      var mockBar = el('div', { style: { display: 'flex', height: '28px', borderRadius: '6px', overflow: 'hidden', gap: '2px', marginBottom: '8px' } });
      if (computed.boundaryPct > 0) mockBar.appendChild(el('div', { style: { width: computed.boundaryPct + '%', background: 'var(--green)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--mono)', fontSize: '11px', fontWeight: '700', color: '#1a1a1e' } }, computed.boundaryPct + '% boundary'));
      if (computed.internalPct > 0) mockBar.appendChild(el('div', { style: { width: computed.internalPct + '%', background: 'var(--accent)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--mono)', fontSize: '11px', fontWeight: '700', color: '#1a1a1e' } }, computed.internalPct + '% internal'));
      if (computed.unclassifiedMockPct > 0) mockBar.appendChild(el('div', { style: { width: computed.unclassifiedMockPct + '%', background: 'var(--bg-elevated)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--mono)', fontSize: '11px', color: 'var(--text-dim)' } }, computed.unclassifiedMockPct + '%'));
      patSection.appendChild(mockBar);
      patSection.appendChild(el('p', { style: { fontSize: '12px', color: 'var(--text-dim)' } }, 'Boundary mocks (network, database) are standard. Internal mocks couple tests to implementation wiring.'));

      if (computed.hasMostMocked) {
        patSection.appendChild(makeDetails('Show most-mocked types', computed.mostMockedTypes.map(function(m) { return m.name + ' (' + m.count + 'x) ' + (m.placementLabel || ''); })));
      }
    }

    // Git signals (collapsible)
    if (computed.hasGitSignals) {
      patSection.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '10px', color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '1.5px', marginBottom: '10px', marginTop: '28px' } }, 'Git Signals'));
      var gitMetrics = el('div', { className: 'metrics' });
      gitMetrics.appendChild(makeMetricCard(computed.gitHighCoChange > 0 ? 'warn' : 'good', computed.gitHighCoChange, 'high co-change ' + pluralize('pair', computed.gitHighCoChange)));
      gitMetrics.appendChild(makeMetricCard(computed.gitStructural > 0 ? 'bad' : 'good', computed.gitStructural, 'break on small tweaks'));
      gitMetrics.appendChild(makeMetricCard('neutral', computed.gitHighChurn, 'high-churn tests'));
      patSection.appendChild(gitMetrics);
      if (computed.gitCoChangeExamples.length > 0) {
        patSection.appendChild(makeDetails('Show co-change examples', computed.gitCoChangeExamples));
      }
      if (computed.gitStructuralExamples.length > 0) {
        patSection.appendChild(makeDetails('Show structural coupling examples', computed.gitStructuralExamples));
      }
    }

    container.appendChild(patSection);
  }

  // Footer
  container.appendChild(el('div', { className: 'footer', innerHTML: 'Generated by Confidence &mdash; ' + timestamp }));

  // --- Helper functions for building common elements ---
  function makeMetricCard(cls, value, label) {
    var card = el('div', { className: 'metric-card ' + cls });
    card.appendChild(el('div', { className: 'value' }, String(value)));
    card.appendChild(el('div', { className: 'label' }, label));
    return card;
  }

  function makeLegendItem(color, text, pctText) {
    var item = el('div', { className: 'chart-legend-item' });
    item.appendChild(el('div', { className: 'dot', style: { background: color } }));
    item.appendChild(el('span', null, text));
    item.appendChild(el('span', { className: 'pct' }, pctText));
    return item;
  }

  function makeDetails(summary, items) {
    var details = el('details', { style: { marginTop: '12px' } });
    details.appendChild(el('summary', { style: { fontFamily: 'var(--mono)', fontSize: '12px', color: 'var(--text-dim)', cursor: 'pointer', userSelect: 'none' } }, summary));
    var content = el('div', { style: { marginTop: '8px', paddingLeft: '12px', borderLeft: '2px solid var(--bg-elevated)' } });
    items.forEach(function(item) {
      content.appendChild(el('div', { style: { fontFamily: 'var(--mono)', fontSize: '12px', color: 'var(--text-muted)', padding: '3px 0' } }, item));
    });
    details.appendChild(content);
    return details;
  }

  // --- Charts (after DOM is built) ---
  initCharts();

  function initCharts() {
    if (typeof Chart === 'undefined') return;

    Chart.defaults.color = '#8a8680';
    Chart.defaults.font.family = "'DM Sans', sans-serif";
    Chart.defaults.plugins.legend.display = false;
    Chart.defaults.plugins.tooltip.backgroundColor = '#222226';
    Chart.defaults.plugins.tooltip.titleColor = '#e8e4df';
    Chart.defaults.plugins.tooltip.bodyColor = '#8a8680';
    Chart.defaults.plugins.tooltip.borderColor = '#2a2a2e';
    Chart.defaults.plugins.tooltip.borderWidth = 1;
    Chart.defaults.plugins.tooltip.cornerRadius = 8;
    Chart.defaults.plugins.tooltip.padding = 10;
    Chart.defaults.plugins.tooltip.titleFont = { family: "'JetBrains Mono', monospace", weight: '700' };

    var donutOpts = {
      cutout: '68%',
      responsive: true,
      maintainAspectRatio: true,
      plugins: { legend: { display: false }, tooltip: { enabled: true } },
      animation: { animateRotate: true, duration: 1200 }
    };

    // Hero donut chart
    if (computed.hasStyle) {
      var heroCanvas = document.getElementById('heroDonut');
      if (heroCanvas) {
        var heroData = [computed.behavioralPct, computed.structuralPct];
        var heroColors = ['#5cb87a', '#d4a24e'];
        var heroLabels = ['Behavioral', 'Structural'];
        var heroCounts = [computed.behavioralCount, computed.structuralCount];
        if (computed.unclassifiedPct > 0) {
          heroData.push(computed.unclassifiedPct);
          heroColors.push('#5a5854');
          heroLabels.push('Unclassified');
          heroCounts.push(computed.unclassifiedCount);
        }
        new Chart(heroCanvas, {
          type: 'doughnut',
          data: {
            labels: heroLabels,
            datasets: [{
              data: heroData,
              backgroundColor: heroColors,
              borderWidth: 0,
              hoverOffset: 6
            }]
          },
          options: {
            responsive: true,
            maintainAspectRatio: true,
            cutout: '72%',
            plugins: {
              legend: { display: false },
              tooltip: {
                callbacks: {
                  label: function(ctx) {
                    var idx = ctx.dataIndex;
                    return heroLabels[idx] + ': ' + heroData[idx] + '% (' + heroCounts[idx] + ' methods)';
                  }
                }
              }
            },
            animation: { animateRotate: true, duration: 1200 }
          }
        });
      }
    }

    // Mock placement chart
    if (computed.hasMockPlacement) {
      var mpCanvas = document.getElementById('mockPlacementChart');
      if (mpCanvas) {
        new Chart(mpCanvas, {
          type: 'doughnut',
          data: {
            labels: ['Boundary', 'Internal', 'Unclassified'],
            datasets: [{
              data: [computed.boundaryPct, computed.internalPct, computed.unclassifiedMockPct],
              backgroundColor: ['#5cb87a', '#d4a24e', '#5a5854'],
              borderWidth: 0,
              hoverOffset: 6
            }]
          },
          options: donutOpts
        });
      }
    }

    // Trend charts
    if (computed.hasTrends) {
      var lineDefaults = { tension: 0.35, pointRadius: 5, pointHoverRadius: 7, pointBorderColor: '#1a1a1e', pointBorderWidth: 2, borderWidth: 2.5, fill: true };
      var axisDefaults = { grid: { color: 'rgba(90,88,84,0.15)', drawBorder: false }, ticks: { font: { family: "'JetBrains Mono', monospace", size: 11 }, color: '#8a8680' } };

      // Chart 1: Suite Growth
      var growthCanvas = document.getElementById('trendGrowthChart');
      if (growthCanvas) {
        new Chart(growthCanvas, {
          type: 'line',
          data: {
            labels: computed.trendLabels,
            datasets: [Object.assign({
              label: 'Test Methods',
              data: computed.trendTestCounts,
              borderColor: '#e8e4df',
              backgroundColor: 'rgba(232, 228, 223, 0.1)',
              pointBackgroundColor: '#e8e4df'
            }, lineDefaults)]
          },
          options: {
            responsive: true, maintainAspectRatio: false,
            plugins: { legend: { display: false } },
            scales: {
              x: axisDefaults,
              y: Object.assign({}, axisDefaults, { min: 0, title: { display: true, text: 'Test Methods', color: '#5a5854', font: { size: 10 } } })
            },
            animation: { duration: 1200, easing: 'easeOutQuart' }
          }
        });
      }

      // Chart 2: Quality Direction
      var behMin = Math.floor(Math.min.apply(null, computed.trendBehavioralPcts) / 5) * 5 - 5;
      var behMax = Math.ceil(Math.max.apply(null, computed.trendBehavioralPcts) / 5) * 5 + 5;
      var strMax = Math.ceil(Math.max.apply(null, computed.trendStructuralRates) / 5) * 5 + 5;

      var qualityCanvas = document.getElementById('trendQualityChart');
      if (qualityCanvas) {
        new Chart(qualityCanvas, {
          type: 'line',
          data: {
            labels: computed.trendLabels,
            datasets: [
              Object.assign({
                label: 'Behavioral %',
                data: computed.trendBehavioralPcts,
                borderColor: '#5cb87a',
                backgroundColor: 'rgba(92, 184, 122, 0.1)',
                pointBackgroundColor: '#5cb87a',
                yAxisID: 'y'
              }, lineDefaults),
              Object.assign({
                label: 'Structural / 100 methods',
                data: computed.trendStructuralRates,
                borderColor: '#d4a24e',
                backgroundColor: 'rgba(212, 162, 78, 0.1)',
                pointBackgroundColor: '#d4a24e',
                yAxisID: 'y1'
              }, lineDefaults)
            ]
          },
          options: {
            responsive: true, maintainAspectRatio: false,
            interaction: { mode: 'index', intersect: false },
            plugins: {
              legend: {
                display: true, position: 'top',
                labels: { color: '#8a8680', usePointStyle: true, pointStyle: 'circle', padding: 16, font: { family: "'DM Sans', sans-serif", size: 11 } }
              }
            },
            scales: {
              x: axisDefaults,
              y: {
                type: 'linear', position: 'left',
                title: { display: true, text: 'Behavioral %', color: '#5cb87a', font: { size: 10 } },
                grid: { color: 'rgba(90,88,84,0.15)', drawBorder: false },
                ticks: { font: { family: "'JetBrains Mono', monospace", size: 11 }, color: '#5cb87a', callback: function(v) { return v + '%'; } },
                min: behMin, max: behMax
              },
              y1: {
                type: 'linear', position: 'right',
                title: { display: true, text: 'Structural Rate', color: '#d4a24e', font: { size: 10 } },
                grid: { display: false },
                ticks: { font: { family: "'JetBrains Mono', monospace", size: 11 }, color: '#d4a24e' },
                min: 0, max: strMax
              }
            },
            animation: { duration: 1200, easing: 'easeOutQuart' }
          }
        });
      }

      // Chart 3: Test Doubles Strategy
      var doublesCanvas = document.getElementById('trendDoublesChart');
      if (doublesCanvas) {
        new Chart(doublesCanvas, {
          type: 'line',
          data: {
            labels: computed.trendLabels,
            datasets: [
              Object.assign({
                label: 'Fakes',
                data: computed.trendFakeCounts,
                borderColor: '#5cb87a',
                backgroundColor: 'rgba(92, 184, 122, 0.1)',
                pointBackgroundColor: '#5cb87a'
              }, lineDefaults),
              Object.assign({
                label: 'Mocks',
                data: computed.trendMockCounts,
                borderColor: '#d4a24e',
                backgroundColor: 'rgba(212, 162, 78, 0.1)',
                pointBackgroundColor: '#d4a24e'
              }, lineDefaults)
            ]
          },
          options: {
            responsive: true, maintainAspectRatio: false,
            interaction: { mode: 'index', intersect: false },
            plugins: {
              legend: {
                display: true, position: 'top',
                labels: { color: '#8a8680', usePointStyle: true, pointStyle: 'circle', padding: 16, font: { family: "'DM Sans', sans-serif", size: 11 } }
              }
            },
            scales: {
              x: axisDefaults,
              y: Object.assign({}, axisDefaults, { title: { display: true, text: 'Count', color: '#5a5854', font: { size: 10 } } })
            },
            animation: { duration: 1200, easing: 'easeOutQuart' }
          }
        });
      }

      // Chart 4: Coverage Progress
      if (computed.hasSurfaceTrend) {
        var covCanvas = document.getElementById('trendCoverageChart');
        if (covCanvas) {
          new Chart(covCanvas, {
            type: 'bar',
            data: {
              labels: computed.trendLabels,
              datasets: [
                {
                  label: 'Tested',
                  data: computed.trendSurfaceTested,
                  backgroundColor: 'rgba(92, 184, 122, 0.8)',
                  borderRadius: 3
                },
                {
                  label: 'Untested',
                  data: computed.trendSurfaceUntested,
                  backgroundColor: 'rgba(199, 92, 92, 0.6)',
                  borderRadius: 3
                }
              ]
            },
            options: {
              responsive: true, maintainAspectRatio: false,
              plugins: {
                legend: {
                  display: true, position: 'top',
                  labels: { color: '#9a9690', usePointStyle: true, pointStyle: 'circle', padding: 16, font: { family: "'DM Sans', sans-serif", size: 11 } }
                }
              },
              scales: {
                x: Object.assign({}, axisDefaults, { stacked: true }),
                y: Object.assign({}, axisDefaults, {
                  stacked: true,
                  title: { display: true, text: 'Surfaces', color: '#706c66', font: { size: 10 } }
                })
              },
              animation: { duration: 1200, easing: 'easeOutQuart' }
            }
          });
        }
      }
    }
  }
})();
