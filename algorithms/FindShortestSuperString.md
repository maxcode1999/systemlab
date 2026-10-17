- Use bitset operations.
- Use same dp state for path reconstruction.
- Iterative dp requires to write dp loop in proper order, subproblem solutions needs to be already added. Recursive DP is better in that sense.
- Always pass dp array from main function. 
```
class Solution {
public:
    int findOverlap(const string& a, const string& b) {
        int n = a.size();
        int m = b.size();

        for (int start = 1; start < n; start++) {
            int i = start, j = 0;
            while (i < n && j < m && a[i] == b[j])
                i++, j++;
            if (i == n)
                return n - start;
        }
        return 0;
    }

    string appendWord(string word, const string& toAdd) {
        int overlap = findOverlap(word, toAdd);
        word.append(toAdd.begin() + overlap, toAdd.end());
        return word;
    }

    int traverse(int cur, int mask, vector<string>& words,
                 vector<vector<int>>& dp, vector<vector<int>>& path, int n) {
        if (dp[cur][mask] != -1)
            return dp[cur][mask];

        bitset<12> remaining(mask);
        remaining.reset(cur);

        int maxOverlap = 0;
        int nextWord = -1;

        for (int i = 0; i < n; i++) {
            if (remaining.test(i)) {
                int overlap = findOverlap(words[i], words[cur]);
                int candidate = overlap +
                    traverse(i, remaining.to_ulong(), words, dp, path, n);

                if (candidate >= maxOverlap) {
                    maxOverlap = candidate;
                    nextWord = i;
                }
            }
        }

        dp[cur][mask] = maxOverlap;
        path[cur][mask] = nextWord;

        return maxOverlap;
    }

    string shortestSuperstring(vector<string>& words) {
        int n = words.size();
        int totalMasks = 1 << n;
        int fullMask = totalMasks - 1;

        vector<vector<int>> dp(n, vector<int>(totalMasks, -1));
        vector<vector<int>> path(n, vector<int>(totalMasks, -1));

        for (int i = 0; i < n; i++)
            traverse(i, fullMask, words, dp, path, n);

        int start = 0;
        for (int i = 1; i < n; i++) {
            if (dp[i][fullMask] >= dp[start][fullMask])
                start = i;
        }

        vector<int> order;
        bitset<12> remaining(fullMask);
        int cur = start;

        while (cur != -1) {
            order.push_back(cur);

            int next = path[cur][remaining.to_ulong()];
            remaining.reset(cur);
            cur = next;
        }

        reverse(order.begin(), order.end());

        string result;
        for (int i : order)
            result = appendWord(result, words[i]);

        return result;
    }
};
```
