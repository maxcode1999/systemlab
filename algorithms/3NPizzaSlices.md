- Invalid case in dp can be kept as 0 or -1 usually.
  
```
class Solution {
public:
    int maxSizeSlices(vector<int>& slices) {
        int n = slices.size() / 3;
        int m = slices.size();

        // dp[i][j][k]: Max sum taking j slices from indices [0, i],
        //              where k = 1 if slice i is taken, otherwise 0.

        vector<vector<vector<int>>> dp(
            m, vector<vector<int>>(n + 1, vector<int>(2, 0))
        );

        dp[0][1][1] = slices[0];

        for (int i = 1; i < m - 1; i++) {
            for (int j = 1; j <= n; j++) {
                dp[i][j][0] = max(dp[i - 1][j][0], dp[i - 1][j][1]);
                dp[i][j][1] = dp[i - 1][j - 1][0] + slices[i];
            }
        }

        vector<vector<vector<int>>> dp2(
            m, vector<vector<int>>(n + 1, vector<int>(2, 0))
        );

        dp2[1][1][1] = slices[1];

        for (int i = 2; i < m; i++) {
            for (int j = 1; j <= n; j++) {
                dp2[i][j][0] = max(dp2[i - 1][j][0], dp2[i - 1][j][1]);
                dp2[i][j][1] = dp2[i - 1][j - 1][0] + slices[i];
            }
        }

        return max({
            dp[m - 2][n][0],
            dp[m - 2][n][1],
            dp2[m - 1][n][0],
            dp2[m - 1][n][1]
        });
    }
};
```
